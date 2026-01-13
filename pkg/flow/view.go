// Package flow: view rendering helpers.
//
// ViewManager is a small template loader/cacher used by the framework to
// render templates according to conventions. It is intentionally minimal
// for the prototype: templates are looked up by name relative to a root
// directory and parsed on first use.
package flow

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sync"

	"github.com/undiegomejia/flow/pkg/i18n"
)

// ViewManager holds template loading configuration and a simple cache.
type ViewManager struct {
	TemplateDir string
	// DefaultLayout is the layout file name (relative to TemplateDir) that
	// should be parsed before the view. Example: "layouts/application.html".
	// If empty, the loader falls back to scanning `layouts/*.html`.
	DefaultLayout string

	// FuncMap contains template functions to register with parsed templates.
	FuncMap template.FuncMap

	// DevMode disables caching and forces reparsing on each Render call when true.
	DevMode bool
	mu      sync.RWMutex
	cache   map[string]*template.Template
}

// NewViewManager constructs a ViewManager which will look for templates in
// templateDir (relative to the working directory).
func NewViewManager(templateDir string) *ViewManager {
	return &ViewManager{TemplateDir: templateDir, cache: make(map[string]*template.Template), FuncMap: template.FuncMap{}}
}

// Render loads (or retrieves from cache) the named template and executes it
// with the provided data into the context's ResponseWriter. Template names
// are file paths relative to TemplateDir without extension, e.g. "users/show".
func (v *ViewManager) Render(name string, data interface{}, ctx *Context) error {
	if v == nil {
		return fmt.Errorf("view manager: nil")
	}
	tpl, err := v.loadTemplate(name)
	if err != nil {
		return err
	}
	// Prefer executing a "content" template (common pattern where views
	// define {{ define "content" }}...{{ end }} and layouts render that
	// via {{ template "content" . }}). If no "content" template exists,
	// fall back to executing the parsed file's base name (e.g. "show.html").
	execName := "content"
	if tpl.Lookup(execName) == nil {
		execName = filepath.Base(name) + ".html"
	}
	// Clone template so we can safely register a per-request FuncMap without
	// mutating the cached parsed templates. Provide a convenient template
	// function T(key, ...) that resolves translations from the request
	// context using pkg/i18n.
	clone, err := tpl.Clone()
	if err != nil {
		return fmt.Errorf("clone template: %w", err)
	}
	clone = clone.Funcs(template.FuncMap{
		"T": func(key string, args ...interface{}) string {
			return i18n.TFromContext(ctx.R.Context(), key, args...)
		},
	})
	return ctx.RenderTemplate(clone, execName, data)
}

func (v *ViewManager) loadTemplate(name string) (*template.Template, error) {
	// If not in dev mode, try cache first.
	if !v.DevMode {
		v.mu.RLock()
		t, ok := v.cache[name]
		v.mu.RUnlock()
		if ok {
			return t, nil
		}
	}

	// build list of candidate files: default layout (if set), layouts, partials, shared, then the view
	var files []string

	// if a DefaultLayout is specified, prefer it first
	if v.DefaultLayout != "" {
		defPath := filepath.Join(v.TemplateDir, v.DefaultLayout)
		if _, err := os.Stat(defPath); err == nil {
			files = append(files, defPath)
		}
	} else {
		// collect layouts (prefer application/layout order)
		layoutGlob := filepath.Join(v.TemplateDir, "layouts", "*.html")
		if lays, _ := filepath.Glob(layoutGlob); len(lays) > 0 {
			files = append(files, lays...)
		}
	}

	// collect partials
	partialGlob := filepath.Join(v.TemplateDir, "partials", "*.html")
	if parts, _ := filepath.Glob(partialGlob); len(parts) > 0 {
		files = append(files, parts...)
	}

	// collect shared helpers (optional)
	sharedGlob := filepath.Join(v.TemplateDir, "shared", "*.html")
	if sh, _ := filepath.Glob(sharedGlob); len(sh) > 0 {
		files = append(files, sh...)
	}

	// finally add the view file itself
	viewPath := filepath.Join(v.TemplateDir, name+".html")
	if _, err := os.Stat(viewPath); err != nil {
		return nil, fmt.Errorf("view file not found: %s", viewPath)
	}
	files = append(files, viewPath)

	// parse template set and register FuncMap if provided
	tpl := template.New(filepath.Base(viewPath))
	// Ensure a baseline FuncMap exists and include a noop T function so templates
	// that reference T can be parsed. The per-request Render() will clone and
	// override T with a real implementation.
	baseFuncs := template.FuncMap{}
	if v.FuncMap != nil {
		for k, f := range v.FuncMap {
			baseFuncs[k] = f
		}
	}
	if _, ok := baseFuncs["T"]; !ok {
		baseFuncs["T"] = func(key string, args ...interface{}) string { return key }
	}
	if len(baseFuncs) > 0 {
		tpl = tpl.Funcs(baseFuncs)
	}
	parsed, err := tpl.ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("parse templates %v: %w", files, err)
	}

	if !v.DevMode {
		v.mu.Lock()
		v.cache[name] = parsed
		v.mu.Unlock()
	}
	return parsed, nil
}

// SetDefaultLayout sets the default layout file (relative to TemplateDir).
func (v *ViewManager) SetDefaultLayout(layout string) {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.DefaultLayout = layout
	// clear cache to ensure layout change takes effect
	v.cache = make(map[string]*template.Template)
	v.mu.Unlock()
}

// SetFuncMap registers template functions to be available during parsing.
// Changing the FuncMap clears the cache so new functions are available.
func (v *ViewManager) SetFuncMap(m template.FuncMap) {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.FuncMap = m
	v.cache = make(map[string]*template.Template)
	v.mu.Unlock()
}

// SetDevMode toggles development mode. When true templates are reparsed on
// every Render call and caching is disabled.
func (v *ViewManager) SetDevMode(dev bool) {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.DevMode = dev
	if dev {
		// clear cache when entering dev mode
		v.cache = make(map[string]*template.Template)
	}
	v.mu.Unlock()
}
