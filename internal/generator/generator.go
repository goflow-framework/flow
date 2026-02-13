package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// generateFile renders tmpl with data and writes it to dstPath. It will
// create directories if necessary and will not overwrite existing files
// unless overwrite is true.
func generateFile(tmplStr string, data interface{}, dstPath string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(dstPath); err == nil {
			return fmt.Errorf("file exists: %s", dstPath)
		}
	}
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	t, err := template.New("tpl").Funcs(template.FuncMap{
		"ToLower": strings.ToLower,
	}).Parse(tmplStr)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(dstPath, buf.Bytes(), 0o644)
}

// GenerateController creates a controller file at the target project path.
// name should be the base controller name (eg. "users").
func GenerateController(projectRoot, name string) (string, error) {
	return GenerateControllerWithOptions(projectRoot, name, GenOptions{})
}

// GenOptions controls generator behavior used by CLI flags.
type GenOptions struct {
	Force          bool // overwrite existing files
	SkipMigrations bool // don't generate migration files
	NoViews        bool // don't generate view files
	NoI18n         bool // don't generate i18n translation files
}

// GenerateControllerWithOptions generates a controller honoring options.
func GenerateControllerWithOptions(projectRoot, name string, opts GenOptions) (string, error) {
	cname := Title(name) + "Controller"
	dst := filepath.Join(projectRoot, "app", "controllers", name+"_controller.go")
	data := map[string]string{
		"Package":    "controllers",
		"Controller": cname,
		"Name":       name,
	}
	return dst, generateFile(controllerTmpl, data, dst, opts.Force)
}

// GenerateModel creates a simple model file under app/models.
func GenerateModel(projectRoot, name string, fields ...string) (string, error) {
	return GenerateModelWithOptions(projectRoot, name, GenOptions{}, fields...)
}

// GenerateModelWithOptions generates a model and writes the model file, honoring options.
func GenerateModelWithOptions(projectRoot, name string, opts GenOptions, fields ...string) (string, error) {
	mname := Title(name)
	dst := filepath.Join(projectRoot, "app", "models", strings.ToLower(name)+".go")

	// parse fields and build struct lines and migration columns using FieldSpec
	var fieldsCodeLines []string
	var columnsLines []string
	needTime := false
	specs, err := ParseFields(fields)
	if err != nil {
		return dst, err
	}
	for _, fs := range specs {
		if strings.Contains(fs.GoType, "time.Time") || strings.Contains(fs.GoType, "*time.Time") {
			needTime = true
		}
		// struct tag: bun and json; use omitempty for nullable
		jsonTag := fs.Name
		if fs.Nullable {
			jsonTag = jsonTag + ",omitempty"
		}
		tag := fmt.Sprintf("`bun:\"%s\" json:\"%s\"`", fs.Name, jsonTag)
		fieldsCodeLines = append(fieldsCodeLines, fmt.Sprintf("    %s %s %s", fs.GoName, fs.GoType, tag))

		// column SQL line (skip id/created/updated handled separately)
		notnull := ""
		if !fs.Nullable {
			notnull = " NOT NULL"
		}
		colLine := fmt.Sprintf("    %s %s%s", fs.Name, fs.SQLType, notnull)
		if fs.Default != nil {
			colLine = colLine + " DEFAULT " + *fs.Default
		}
		if fs.Unique {
			colLine = colLine + " UNIQUE"
		}
		columnsLines = append(columnsLines, colLine)
	}

	fieldsCode := ""
	if len(fieldsCodeLines) > 0 {
		fieldsCode = strings.Join(fieldsCodeLines, "\n") + "\n"
	}
	cols := ""
	if len(columnsLines) > 0 {
		cols = ",\n" + strings.Join(columnsLines, ",\n")
	}

	extraImports := ""
	if needTime {
		extraImports = "\n    \"time\""
	}

	data := map[string]string{
		"Package":      "models",
		"Model":        mname,
		"FieldsCode":   fieldsCode,
		"Columns":      cols,
		"ExtraImports": extraImports,
	}

	return dst, generateFile(bunModelTmpl, data, dst, opts.Force)
}

// GenerateScaffold generates controller + model + basic views.
func GenerateScaffold(projectRoot, name string, fields ...string) ([]string, error) {
	return GenerateScaffoldWithOptions(projectRoot, name, GenOptions{}, fields...)
}

// GenerateScaffoldWithOptions generates controller + model + basic views and migrations honoring options.
func GenerateScaffoldWithOptions(projectRoot, name string, opts GenOptions, fields ...string) ([]string, error) {
	var created []string
	// controller
	cpath, err := GenerateControllerWithOptions(projectRoot, name, opts)
	if err != nil {
		return created, err
	}
	created = append(created, cpath)

	// model
	mpath, err := GenerateModelWithOptions(projectRoot, name, opts, fields...)
	if err != nil {
		return created, err
	}
	created = append(created, mpath)

	// views
	if !opts.NoViews {
		viewsDir := filepath.Join(projectRoot, "app", "views", name)
		if err := os.MkdirAll(viewsDir, 0o750); err != nil {
			return created, err
		}
		idxPath := filepath.Join(viewsDir, "index.html")
		showPath := filepath.Join(viewsDir, "show.html")
		newPath := filepath.Join(viewsDir, "new.html")
		editPath := filepath.Join(viewsDir, "edit.html")
		// write using templates (use opts.Force for overwrite)
		_ = generateFile(viewIndexTmpl, nil, idxPath, opts.Force)
		_ = generateFile(viewShowTmpl, nil, showPath, opts.Force)
		_ = generateFile(viewNewTmpl, nil, newPath, opts.Force)
		_ = generateFile(viewEditTmpl, nil, editPath, opts.Force)
		created = append(created, idxPath, showPath, newPath, editPath)
	}

	// scaffold i18n translations (minimal en.yaml)
	if !opts.NoI18n {
		i18nDir := filepath.Join(projectRoot, "app", "i18n")
		if err := os.MkdirAll(i18nDir, 0o750); err != nil {
			return created, err
		}
		i18nPath := filepath.Join(i18nDir, "en.yaml")
		_ = generateFile(i18nEnTmpl, nil, i18nPath, opts.Force)
		created = append(created, i18nPath)
	}

	// migrations
	if !opts.SkipMigrations {
		migDir := filepath.Join(projectRoot, "db", "migrate")
		if err := os.MkdirAll(migDir, 0o750); err != nil {
			return created, err
		}
		ts := TimestampNow()
		table := TableName(name)
		upName := fmt.Sprintf("%s_create_%s.up.sql", ts, table)
		downName := fmt.Sprintf("%s_create_%s.down.sql", ts, table)
		upPath := filepath.Join(migDir, upName)
		downPath := filepath.Join(migDir, downName)

		// compute columns SQL for migration based on fields
		var columnsLines []string
		specs2, err := ParseFields(fields)
		if err != nil {
			return created, err
		}
		for _, fs := range specs2 {
			notnull := ""
			if !fs.Nullable {
				notnull = " NOT NULL"
			}
			col := fmt.Sprintf("    %s %s%s", fs.Name, fs.SQLType, notnull)
			if fs.Default != nil {
				col = col + " DEFAULT " + *fs.Default
			}
			if fs.Unique {
				col = col + " UNIQUE"
			}
			columnsLines = append(columnsLines, col)
		}
		cols := ""
		if len(columnsLines) > 0 {
			cols = ",\n" + strings.Join(columnsLines, ",\n")
		}

		// build extras: indexes (CREATE INDEX) and corresponding DROP INDEX for down
		var extrasUpLines []string
		var extrasDownLines []string
		for _, fs := range specs2 {
			if fs.Index {
				idxName := fmt.Sprintf("idx_%s_%s", table, fs.Name)
				extrasUpLines = append(extrasUpLines, fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s(%s);", idxName, table, fs.Name))
				extrasDownLines = append(extrasDownLines, fmt.Sprintf("DROP INDEX IF EXISTS %s;", idxName))
			}
		}
		extrasUp := ""
		if len(extrasUpLines) > 0 {
			extrasUp = strings.Join(extrasUpLines, "\n") + "\n"
		}
		extrasDown := ""
		if len(extrasDownLines) > 0 {
			extrasDown = strings.Join(extrasDownLines, "\n") + "\n"
		}

		// render migration templates (include extras for indexes)
		upData := map[string]string{"Timestamp": ts, "Table": table, "Columns": cols, "ExtrasUp": extrasUp}
		downData := map[string]string{"Timestamp": ts, "Table": table, "ExtrasDown": extrasDown}
		if err := generateFile(migrationUpTmpl, upData, upPath, opts.Force); err != nil {
			return created, err
		}
		if err := generateFile(migrationDownTmpl, downData, downPath, opts.Force); err != nil {
			return created, err
		}
		created = append(created, upPath, downPath)
	}

	// small delay to avoid duplicate timestamps when called rapidly
	time.Sleep(1 * time.Second)
	return created, nil
}

// GenerateAdminWithOptions generates admin controller, views, layout, CSS and a small README.
func GenerateAdminWithOptions(projectRoot, name string, opts GenOptions, fields ...string) ([]string, error) {
	var created []string

	// controller
	cname := Title(name) + "AdminController"
	dst := filepath.Join(projectRoot, "app", "controllers", "admin", strings.ToLower(name)+"_admin_controller.go")
	data := map[string]string{
		"Package":    "controllers",
		"Controller": cname,
		"Name":       name,
		"Title":      Title(name),
	}
	if err := generateFile(adminControllerTmpl, data, dst, opts.Force); err != nil {
		return created, err
	}
	created = append(created, dst)

	// views directory
	viewsDir := filepath.Join(projectRoot, "app", "views", "admin", name)
	if err := os.MkdirAll(viewsDir, 0o750); err != nil {
		return created, err
	}
	idxPath := filepath.Join(viewsDir, "index.html")
	showPath := filepath.Join(viewsDir, "show.html")
	newPath := filepath.Join(viewsDir, "new.html")
	editPath := filepath.Join(viewsDir, "edit.html")
	_ = generateFile(viewAdminIndexTmpl, map[string]string{"Title": Title(name), "Name": name}, idxPath, opts.Force)
	_ = generateFile(viewAdminShowTmpl, map[string]string{"Title": Title(name), "Name": name}, showPath, opts.Force)
	_ = generateFile(viewAdminNewTmpl, map[string]string{"Title": Title(name), "Name": name}, newPath, opts.Force)
	_ = generateFile(viewAdminEditTmpl, map[string]string{"Title": Title(name), "Name": name}, editPath, opts.Force)
	created = append(created, idxPath, showPath, newPath, editPath)

	// scaffold i18n translations (minimal en.yaml)
	if !opts.NoI18n {
		i18nDir := filepath.Join(projectRoot, "app", "i18n")
		if err := os.MkdirAll(i18nDir, 0o750); err != nil {
			return created, err
		}
		i18nPath := filepath.Join(i18nDir, "en.yaml")
		_ = generateFile(i18nEnTmpl, nil, i18nPath, opts.Force)
		created = append(created, i18nPath)
	}

	// layout
	layoutsDir := filepath.Join(projectRoot, "app", "views", "admin", "layouts")
	if err := os.MkdirAll(layoutsDir, 0o750); err != nil {
		return created, err
	}
	layoutPath := filepath.Join(layoutsDir, "admin.html")
	// some layout templates may reference a "content" sub-template; ensure execution
	// succeeds by providing an empty definition for it when rendering for the generator.
	layoutTmpl := adminLayoutTmpl + "{{define \"content\"}}{{end}}"
	_ = generateFile(layoutTmpl, map[string]string{"Title": Title(name)}, layoutPath, opts.Force)
	created = append(created, layoutPath)

	// assets (css)
	assetsDir := filepath.Join(projectRoot, "app", "assets", "admin")
	if err := os.MkdirAll(assetsDir, 0o750); err != nil {
		return created, err
	}
	cssPath := filepath.Join(assetsDir, "admin.css")
	_ = generateFile(adminCSSTmpl, nil, cssPath, opts.Force)
	created = append(created, cssPath)

	// README
	adminDir := filepath.Join(projectRoot, "app", "admin")
	if err := os.MkdirAll(adminDir, 0o750); err != nil {
		return created, err
	}
	readmePath := filepath.Join(adminDir, "README.md")
	_ = generateFile(adminReadmeTmpl, map[string]string{"Title": Title(name)}, readmePath, opts.Force)
	created = append(created, readmePath)

	return created, nil
}

// GenerateAuthWithOptions generates auth scaffolding: User model (via model
// generator), auth controller, views, middleware and a small README.
func GenerateAuthWithOptions(projectRoot string, opts GenOptions, fields ...string) ([]string, error) {
	var created []string

	// default fields if none provided
	if len(fields) == 0 {
		fields = []string{"email:string,unique", "password_hash:string", "role:string"}
	}

	// create User model (uses existing model generator which also creates migrations)
	mpath, err := GenerateModelWithOptions(projectRoot, "User", opts, fields...)
	if err != nil {
		return created, err
	}
	created = append(created, mpath)

	// controller
	dst := filepath.Join(projectRoot, "app", "controllers", "auth_controller.go")
	data := map[string]string{"Package": "controllers", "Name": "auth", "Title": "Auth"}
	if err := generateFile(authControllerTmpl, data, dst, opts.Force); err != nil {
		return created, err
	}
	created = append(created, dst)

	// views
	viewsDir := filepath.Join(projectRoot, "app", "views", "auth")
	if err := os.MkdirAll(viewsDir, 0o750); err != nil {
		return created, err
	}
	loginPath := filepath.Join(viewsDir, "login.html")
	_ = generateFile(viewAuthLoginTmpl, map[string]string{"Title": "Login"}, loginPath, opts.Force)
	created = append(created, loginPath)

	// middleware
	mwDir := filepath.Join(projectRoot, "app", "middleware")
	if err := os.MkdirAll(mwDir, 0o750); err != nil {
		return created, err
	}
	mwPath := filepath.Join(mwDir, "auth.go")
	_ = generateFile(authMiddlewareTmpl, nil, mwPath, opts.Force)
	created = append(created, mwPath)

	// README
	adminDir := filepath.Join(projectRoot, "app", "auth")
	if err := os.MkdirAll(adminDir, 0o750); err != nil {
		return created, err
	}
	readmePath := filepath.Join(adminDir, "README.md")
	_ = generateFile(authReadmeTmpl, map[string]string{"Title": "Auth"}, readmePath, opts.Force)
	created = append(created, readmePath)

	return created, nil
}
