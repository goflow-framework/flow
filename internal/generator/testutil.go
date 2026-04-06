package generator

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TempModule creates a temporary Go module for generator tests. It creates a
// temp directory via t.TempDir(), writes a go.mod that pins the repo module
// via an absolute replace, and returns the project directory and the
// generated module name. It also logs key `go env` values to help debug
// toolchain issues in CI.
func TempModule(t *testing.T) (projDir, moduleName string) {
	t.Helper()
	proj := t.TempDir()
	uid := filepath.Base(proj)
	repo := findRepoRoot()
	modName, err := readModuleName(repo)
	if err != nil {
		t.Fatalf("read module name: %v", err)
	}
	moduleName = modName + "/examples/" + uid
	if err := WriteTempGoMod(proj, moduleName, false); err != nil {
		t.Fatalf("WriteTempGoMod failed: %v", err)
	}
	// Log relevant go env values to aid CI debugging when tests fail.
	if out, err := exec.Command("go", "env", "GOMODCACHE", "GOPROXY", "GOSUMDB").CombinedOutput(); err == nil {
		t.Logf("go env: %s", string(out))
	}
	return proj, moduleName
}

// findRepoRoot walks up from the current working directory until it finds a go.mod
// and returns the directory path. If none is found it returns the original cwd.
func findRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	cur := wd
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur || parent == "" {
			break
		}
		cur = parent
	}
	return wd
}

// readModuleName reads the module name from repo's go.mod
func readModuleName(repo string) (string, error) {
	bm, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		return "", err
	}
	s := string(bm)
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", nil
}

// readGoVersion reads the "go X.Y" line from repo's go.mod and returns the version
func readGoVersion(repo string) (string, error) {
	bm, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		return "", err
	}
	s := string(bm)
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go ")), nil
		}
	}
	// Prefer a modern Go version for generated test modules to avoid the
	// toolchain automatically switching or choosing an older default that
	// can make builds brittle across CI environments. Use 1.24 as a sensible
	// baseline; callers can still override by reading the repo go.mod.
	return "1.24", nil
}

// RunCmdCombined runs a command in dir and returns its combined output. If the
// command fails, the returned error will include `go env` output when possible
// to make debugging failures in CI/toolchains easier.
func RunCmdCombined(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	// If invoking the go tool, use a local GOMODCACHE inside the temp project to
	// avoid polluting the user's module cache and to make behavior reproducible
	// across CI environments.
	if name == "go" {
		gomodcache := filepath.Join(dir, ".gomodcache")
		// ensure the dir exists
		_ = os.MkdirAll(gomodcache, 0o755)

		// make the gomodcache writable so that test harnesses can remove it
		// during cleanup even if the go tool leaves files with restrictive
		// permissions. This is a best-effort chmod; ignore errors.
		_ = os.Chmod(gomodcache, 0o777)
		env := os.Environ()
		// Ensure reproducible module cache location for tests.
		env = append(env, "GOMODCACHE="+gomodcache)

		// If GOPROXY or GOSUMDB are not set in the test environment, set
		// conservative defaults so tests are less likely to fail due to
		// environment-specific proxy/sumdb configuration. We only set them
		// when absent to avoid overriding intentional CI settings.
		if os.Getenv("GOPROXY") == "" {
			env = append(env, "GOPROXY=https://proxy.golang.org,direct")
		}
		if os.Getenv("GOSUMDB") == "" {
			env = append(env, "GOSUMDB=sum.golang.org")
		}

		// Opt-in: allow test runs to request a writable modcache via an
		// environment variable. This avoids forcing -modcacherw on all
		// developers (some go toolchains may not accept it). CI can set
		// FLOW_TEST_ALLOW_MODCACHE_RW=1 when appropriate.
		if v := os.Getenv("FLOW_TEST_ALLOW_MODCACHE_RW"); v == "1" || strings.EqualFold(v, "true") {
			env = append(env, "GOFLAGS=-modcacherw")
		}

		cmd.Env = env

		// Diagnostics: when FLOW_TEST_DIAG=1 is set, print the exact go
		// command and the relevant env vars so we can trace where flags
		// like -modcacherw originate.
		if os.Getenv("FLOW_TEST_DIAG") == "1" {
			// Print to stderr so test logs capture it reliably.
			fmt.Fprintf(os.Stderr, "[gen-testutil] running: %s %s\n", name, strings.Join(args, " "))
			for _, e := range cmd.Env {
				if strings.HasPrefix(e, "GOFLAGS=") || strings.HasPrefix(e, "GOMODCACHE=") || strings.HasPrefix(e, "GOPROXY=") || strings.HasPrefix(e, "GOSUMDB=") {
					fmt.Fprintln(os.Stderr, "[gen-testutil] env:", e)
				}
			}
		}
	}

	out, err := cmd.CombinedOutput()
	if err == nil {
		// try to relax permissions inside the gomodcache to avoid permission
		// denied errors when the test framework attempts to recursively remove
		// the temp directory. Best-effort: ignore any errors here.
		if name == "go" {
			gomodcache := filepath.Join(dir, ".gomodcache")
			_ = filepath.WalkDir(gomodcache, func(p string, d os.DirEntry, e error) error {
				if e != nil {
					return nil
				}
				// Skip symlinks: do not follow/chmod symlinks to avoid TOCTOU.
				if (d.Type() & os.ModeSymlink) != 0 {
					return nil
				}
				if fi, err := d.Info(); err == nil {
					if fi.Mode()&os.ModeSymlink != 0 {
						return nil
					}
				}
				if d.IsDir() {
					_ = os.Chmod(p, 0o777) // #nosec G122 -- path comes from WalkDir on a controlled temp dir; symlinks skipped above.
					return nil
				}
				_ = os.Chmod(p, 0o666) // #nosec G122 -- path comes from WalkDir on a controlled temp dir; symlinks skipped above.
				return nil
			})
		}
		return out, nil
	}
	// try to capture go env to aid debugging. When possible, run the
	// `go env` capture with the same environment we used for the failing
	// command so the diagnostic output matches what the tool saw.
	goEnvCmd := exec.Command("go", "env")
	goEnvGP := exec.Command("go", "env", "GOPROXY", "GOSUMDB")
	if cmd.Env != nil {
		goEnvCmd.Env = cmd.Env
		goEnvGP.Env = cmd.Env
	}
	if envOut, e := goEnvCmd.CombinedOutput(); e == nil {
		// Also include GOPROXY and GOSUMDB explicitly (helpful for CI debugging).
		gpOut, _ := goEnvGP.CombinedOutput()
		// best-effort relax perms on failure path too
		if name == "go" {
			gomodcache := filepath.Join(dir, ".gomodcache")
			_ = filepath.WalkDir(gomodcache, func(p string, d os.DirEntry, e error) error {
				if e != nil {
					return nil
				}
				// Skip symlinks: do not follow/chmod symlinks to avoid TOCTOU.
				if (d.Type() & os.ModeSymlink) != 0 {
					return nil
				}
				if fi, err := d.Info(); err == nil {
					if fi.Mode()&os.ModeSymlink != 0 {
						return nil
					}
				}
				if d.IsDir() {
					_ = os.Chmod(p, 0o777) // #nosec G122 -- path comes from WalkDir on a controlled temp dir; symlinks skipped above.
					return nil
				}
				_ = os.Chmod(p, 0o666) // #nosec G122 -- path comes from WalkDir on a controlled temp dir; symlinks skipped above.
				return nil
			})
		}
		return out, fmt.Errorf("%v\noutput: %s\n--- go env ---\n%s\n--- go env GOPROXY GOSUMDB ---\n%s", err, string(out), string(envOut), string(gpOut))
	}
	return out, fmt.Errorf("%v\noutput: %s", err, string(out))
}

// WriteTempGoMod writes a minimal go.mod into projDir for moduleName. The
// go version is taken from the repository root's go.mod and the repo module
// is added as a replace directive using an absolute path so temporary
// modules resolve local packages reliably across toolchains.
func WriteTempGoMod(projDir, moduleName string, replaceSelf bool) error {
	repo := findRepoRoot()
	modName, err := readModuleName(repo)
	if err != nil {
		return fmt.Errorf("read module name: %w", err)
	}
	gov, err := readGoVersion(repo)
	if err != nil {
		// Prefer a modern baseline for generated test modules. Use 1.24
		// to match repository tooling and avoid older-default toolchain
		// behavior that can make CI brittle.
		gov = "1.24"
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		absRepo = repo
	}

	goMod := fmt.Sprintf("module %s\n\ngo %s\n\nrequire %s v0.0.0\n\nreplace %s => %s\n",
		moduleName, gov, modName, modName, absRepo)
	if replaceSelf {
		// point the module name to the absolute temp project path so the
		// go toolchain resolves imports without ambiguity across environments
		// and proxy settings.
		absProj, err := filepath.Abs(projDir)
		if err != nil {
			absProj = projDir
		}
		goMod += fmt.Sprintf("replace %s => %s\n", moduleName, absProj)
	}
	return os.WriteFile(filepath.Join(projDir, "go.mod"), []byte(goMod), 0o644)
}

// WriteTempModule is a small helper to write a minimal temporary go.mod for
// generator tests. It pins the go version to 1.24 and writes an absolute
// replace directive pointing the module requirement to the provided repo
// root. Use t.TempDir() to create the directory and pass it as `dir`.
func WriteTempModule(t *testing.T, dir string, root string) {
	// keep the module name generic; TempModule uses unique module names
	content := fmt.Sprintf("module tempgen\n\ngo 1.24\n\nrequire github.com/undiegomejia/flow v0.0.0\n\nreplace github.com/undiegomejia/flow => %s\n", root)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteTempModule: %v", err)
	}
}

// RunGoCombined is a convenience wrapper around RunCmdCombined for the go tool.
func RunGoCombined(dir string, args ...string) ([]byte, error) {
	return RunCmdCombined(dir, "go", args...)
}

// RunGoOrFail is a test helper that runs the go tool (via RunGoCombined) and
// fails the test with helpful `go env` output when the command returns an
// error. Use this in tests to get immediate diagnostic information without
// duplicating the go env capture logic at each call-site.
func RunGoOrFail(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	out, err := RunGoCombined(dir, args...)
	if err == nil {
		return out
	}
	// try to capture go env to aid debugging
	if envOut, e := exec.Command("go", "env").CombinedOutput(); e == nil {
		// Also include GOPROXY and GOSUMDB explicitly
		gpOut, _ := exec.Command("go", "env", "GOPROXY", "GOSUMDB").CombinedOutput()
		t.Fatalf("go %v failed: %v\noutput: %s\n--- go env ---\n%s\n--- go env GOPROXY GOSUMDB ---\n%s", args, err, string(out), string(envOut), string(gpOut))
	}
	t.Fatalf("go %v failed: %v\noutput: %s", args, err, string(out))
	return nil
}
