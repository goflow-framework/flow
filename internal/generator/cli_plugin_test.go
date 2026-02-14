package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCLI_GeneratePlugin_Sample(t *testing.T) {
	repo := findRepoRoot()
	tmp := t.TempDir()

	// build the CLI binary into the temp dir
	bin := filepath.Join(tmp, "flow-cli")
	if bout, err := RunGoCombined(repo, "build", "-o", bin, "./cmd/flow"); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// run plugin generator 'samplegen'
	cmd := exec.Command(bin, "generate", "plugin", "samplegen", "--target", tmp)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	t.Logf("cmd output: %s", string(out))
	if err != nil {
		t.Fatalf("cli generate plugin failed: %v", err)
	}

	// assert sample file created
	p := filepath.Join(tmp, "SAMPLE_GENERATED.txt")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("sample plugin did not create file: %v", err)
	}
}
