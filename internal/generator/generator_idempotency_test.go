package generator

// generator_idempotency_test.go covers:
//   - Second run without Force returns "file exists" error (no silent overwrites).
//   - Second run with Force=true succeeds and produces byte-identical output.
//   - GenOptions flags (SkipMigrations, NoViews, NoI18n) suppress the right files.
//   - Generated file content contains expected package, struct, and table names.
//   - Table-driven coverage across Controller, Model, and Scaffold generators.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// readFile reads a file and fails the test if it cannot.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile(%q): %v", path, err)
	}
	return b
}

// assertFileContains fails if path does not contain all of the given substrings.
func assertFileContains(t *testing.T, path string, substrings ...string) {
	t.Helper()
	content := string(readFile(t, path))
	for _, s := range substrings {
		if !strings.Contains(content, s) {
			t.Errorf("file %q: expected to contain %q\nactual content:\n%s", path, s, content)
		}
	}
}

// assertFileAbsent fails if path exists.
func assertFileAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file %q to be absent, but it exists", path)
	}
}

// ─── Controller ───────────────────────────────────────────────────────────────

func TestGenerateController_ContentIsCorrect(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	path, err := GenerateController(td, "article")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	assertFileContains(t, path,
		"package controllers",
		"ArticleController",
		"NewArticleController",
	)
}

func TestGenerateController_SecondRunWithoutForce_Errors(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	if _, err := GenerateController(td, "comment"); err != nil {
		t.Fatalf("first run: %v", err)
	}

	_, err := GenerateController(td, "comment")
	if err == nil {
		t.Fatal("expected error on second run without Force, got nil")
	}
	if !strings.Contains(err.Error(), "file exists") {
		t.Errorf("expected 'file exists' error, got: %v", err)
	}
}

func TestGenerateController_ForceOverwrite_ProducesIdenticalContent(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	path, err := GenerateControllerWithOptions(td, "tag", GenOptions{})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := readFile(t, path)

	path2, err := GenerateControllerWithOptions(td, "tag", GenOptions{Force: true})
	if err != nil {
		t.Fatalf("force re-run: %v", err)
	}
	second := readFile(t, path2)

	if string(first) != string(second) {
		t.Errorf("force re-run produced different content:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// ─── Model ────────────────────────────────────────────────────────────────────

func TestGenerateModel_ContentIsCorrect(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	path, err := GenerateModel(td, "product", "name:string", "price:float")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	assertFileContains(t, path,
		"package models",
		"Product",
		"Name",
		"Price",
	)
}

func TestGenerateModel_SecondRunWithoutForce_Errors(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	if _, err := GenerateModel(td, "invoice"); err != nil {
		t.Fatalf("first run: %v", err)
	}

	_, err := GenerateModel(td, "invoice")
	if err == nil {
		t.Fatal("expected error on second run without Force, got nil")
	}
	if !strings.Contains(err.Error(), "file exists") {
		t.Errorf("expected 'file exists' error, got: %v", err)
	}
}

func TestGenerateModel_ForceOverwrite_ProducesIdenticalContent(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	fields := []string{"title:string", "published:bool"}
	path, err := GenerateModelWithOptions(td, "post", GenOptions{}, fields...)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := readFile(t, path)

	path2, err := GenerateModelWithOptions(td, "post", GenOptions{Force: true}, fields...)
	if err != nil {
		t.Fatalf("force re-run: %v", err)
	}
	second := readFile(t, path2)

	if string(first) != string(second) {
		t.Errorf("force re-run produced different content")
	}
}

// ─── Scaffold flags ───────────────────────────────────────────────────────────

func TestGenerateScaffold_SkipMigrations_NoMigrationFilesCreated(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	_, err := GenerateScaffoldWithOptions(td, "order", GenOptions{SkipMigrations: true})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	migDir := filepath.Join(td, "db", "migrate")
	if _, err := os.Stat(migDir); err == nil {
		entries, _ := os.ReadDir(migDir)
		if len(entries) > 0 {
			t.Errorf("expected no migration files with SkipMigrations=true, found %d entries", len(entries))
		}
	}
	// migration dir may not exist at all — that's fine too
}

func TestGenerateScaffold_NoViews_NoViewFilesCreated(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	_, err := GenerateScaffoldWithOptions(td, "category", GenOptions{NoViews: true, SkipMigrations: true})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	viewsDir := filepath.Join(td, "app", "views", "category")
	for _, f := range []string{"index.html", "show.html", "new.html", "edit.html"} {
		assertFileAbsent(t, filepath.Join(viewsDir, f))
	}
}

func TestGenerateScaffold_NoI18n_NoI18nFileCreated(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	_, err := GenerateScaffoldWithOptions(td, "label", GenOptions{NoI18n: true, SkipMigrations: true, NoViews: true})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	assertFileAbsent(t, filepath.Join(td, "app", "i18n", "en.yaml"))
}

func TestGenerateScaffold_DefaultRun_AllFilesPresent(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	created, err := GenerateScaffoldWithOptions(td, "widget", GenOptions{SkipMigrations: true})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	want := map[string]bool{
		filepath.Join(td, "app", "controllers", "widget_controller.go"): false,
		filepath.Join(td, "app", "models", "widget.go"):                 false,
		filepath.Join(td, "app", "views", "widget", "index.html"):       false,
		filepath.Join(td, "app", "views", "widget", "show.html"):        false,
		filepath.Join(td, "app", "views", "widget", "new.html"):         false,
		filepath.Join(td, "app", "views", "widget", "edit.html"):        false,
		filepath.Join(td, "app", "i18n", "en.yaml"):                     false,
	}
	for _, p := range created {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("expected file %q in created list", path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %q on disk: %v", path, err)
		}
	}
}

func TestGenerateScaffold_SecondRunWithoutForce_Errors(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	opts := GenOptions{SkipMigrations: true}
	if _, err := GenerateScaffoldWithOptions(td, "note", opts); err != nil {
		t.Fatalf("first run: %v", err)
	}

	_, err := GenerateScaffoldWithOptions(td, "note", opts)
	if err == nil {
		t.Fatal("expected error on second scaffold run without Force, got nil")
	}
	if !strings.Contains(err.Error(), "file exists") {
		t.Errorf("expected 'file exists' error, got: %v", err)
	}
}

func TestGenerateScaffold_ForceRerun_ControllerContentIdentical(t *testing.T) {
	t.Parallel()
	td := t.TempDir()

	opts := GenOptions{SkipMigrations: true, NoViews: true, NoI18n: true}
	created, err := GenerateScaffoldWithOptions(td, "ticket", opts)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	// find the controller path
	var ctrlPath string
	for _, p := range created {
		if strings.HasSuffix(p, "_controller.go") {
			ctrlPath = p
			break
		}
	}
	if ctrlPath == "" {
		t.Fatal("controller path not found in created files")
	}
	first := readFile(t, ctrlPath)

	opts.Force = true
	created2, err := GenerateScaffoldWithOptions(td, "ticket", opts)
	if err != nil {
		t.Fatalf("force re-run: %v", err)
	}

	var ctrlPath2 string
	for _, p := range created2 {
		if strings.HasSuffix(p, "_controller.go") {
			ctrlPath2 = p
			break
		}
	}
	second := readFile(t, ctrlPath2)
	if string(first) != string(second) {
		t.Errorf("force re-run produced different controller content")
	}
}

// ─── Table-driven: name variations ───────────────────────────────────────────

func TestGenerateController_TableDriven_Names(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		wantController string
	}{
		{"user", "UserController"},
		{"blog_post", "Blog_postController"},
		{"order", "OrderController"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			td := t.TempDir()
			path, err := GenerateController(td, tc.name)
			if err != nil {
				t.Fatalf("GenerateController(%q): %v", tc.name, err)
			}
			assertFileContains(t, path, tc.wantController, "package controllers")
		})
	}
}

func TestGenerateModel_TableDriven_Fields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		modelName  string
		fields     []string
		wantInFile []string
	}{
		{
			modelName:  "event",
			fields:     []string{"title:string", "starts_at:time"},
			wantInFile: []string{"Event", "Title", "Starts_at"},
		},
		{
			modelName:  "review",
			fields:     []string{"score:int", "body:text"},
			wantInFile: []string{"Review", "Score", "Body"},
		},
		{
			modelName:  "photo",
			fields:     []string{},
			wantInFile: []string{"Photo", "package models"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.modelName, func(t *testing.T) {
			t.Parallel()
			td := t.TempDir()
			path, err := GenerateModel(td, tc.modelName, tc.fields...)
			if err != nil {
				t.Fatalf("GenerateModel(%q): %v", tc.modelName, err)
			}
			assertFileContains(t, path, tc.wantInFile...)
		})
	}
}
