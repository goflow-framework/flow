package generator

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	// default to 1.20 if not present
	return "1.20", nil
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
		env := os.Environ()
		env = append(env, "GOMODCACHE="+gomodcache)
		cmd.Env = env
	}

	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	// try to capture go env to aid debugging
	if envOut, e := exec.Command("go", "env").CombinedOutput(); e == nil {
		// Also include GOPROXY and GOSUMDB explicitly (helpful for CI debugging).
		gpOut, _ := exec.Command("go", "env", "GOPROXY", "GOSUMDB").CombinedOutput()
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
		gov = "1.20"
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

// RunGoCombined is a convenience wrapper around RunCmdCombined for the go tool.
func RunGoCombined(dir string, args ...string) ([]byte, error) {
	return RunCmdCombined(dir, "go", args...)
}
