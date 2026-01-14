package generator

import (
	"bufio"
	"os"
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
