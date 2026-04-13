// Package flow: view rendering helpers.
//
// ViewManager is a small template loader/cacher used by the framework to
// render templates according to conventions. It is intentionally minimal
// for the prototype: templates are looked up by name relative to a root
// directory and parsed on first use.
//
// Performance note
// ----------------
// html/template.Clone() performs a deep copy of the entire template set. The
// previous implementation called Clone() on every Render() call even when the
// template was served from the in-memory cache, which meant O(n_templates)
// allocation on every HTTP response.
//
// The fix: each cached template entry owns a sync.Pool whose New function
// produces one clone (with the per-request T func wired up). Render() borrows
// a clone from the pool, executes it, then returns it to the pool. Under load
// most clones are recycled between requests, eliminating repeated allocations.
package flow

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sync"

	"github.com/undiegomejia/flow/pkg/i18n"
)

// templateEntry holds a parsed template together with a pool of ready-to-use
// clones. Each clone has the T translation function pre-wired via a
// *tCtxHolder that is updated before ExecuteTemplate and reset afterwards.
type templateEntry struct {
	tpl  *template.Template
	pool sync.Pool // pool of *templateClone
}

// templateClone is a single clone of a cached template together with the
// holder that lets the T func read the current request context.
type templateClone struct {
	clone  *template.Template
	holder *tCtxHolder
}

// tCtxHolder is a small mutable box that holds the *Context for the duration
// of a single template execution. The T function registered at clone-creation
// time closes over it; Render swaps the value in and out around each Execute
// call so that T always reads the right per-request context.
type tCtxHolder struct {
	mu  sync.Mutex
	ctx *Context
}

func (h *tCtxHolder) set(c *Context) {
	h.mu.Lock()
	h.ctx = c
	h.mu.Unlock()
}

func (h *tCtxHolder) clear() {
	h.mu.Lock()
	h.ctx = nil
	h.mu.Unlock()
}

func (h *tCtxHolder) translate(key string, args ...interface{}) string {
	h.mu.Lock()
	c := h.ctx
	h.mu.Unlock()
	if c == nil {
		if len(args) > 0 {
			return fmt.Sprintf(key, args...)
		}
		return key
	}
	return i18n.TFromContext(c.r.Context(), key, args...)
}

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
	cache   map[string]*templateEntry
}

// NewViewManager constructs a ViewManager which will look for templates in
// templateDir (relative to the working directory).
func NewViewManager(templateDir string) *ViewManager {
	return &ViewManager{TemplateDir: templateDir, cache: make(map[string]*templateEntry), FuncMap: template.FuncMap{}}
}

// Render loads (or retrieves from cache) the named template and executes it
// with the provided data into the context's ResponseWriter. Template names
// are file paths relative to TemplateDir without extension, e.g. "users/show".
func (v *ViewManager) Render(name string, data interface{}, ctx *Context) error {
	if v == nil {
		return fmt.Errorf("view manager: nil")
	}
	entry, err := v.loadTemplate(name)
	if err != nil {
		return err
	}

	// Borrow a pre-cloned template from the pool. The clone already has T
	// wired up via a tCtxHolder; we just need to set the current context on
	// the holder before execution and clear it afterwards.
	tc, _ := entry.pool.Get().(*templateClone)
	if tc == nil {
		return fmt.Errorf("view manager: failed to obtain template clone for %q", name)
	}
	tc.holder.set(ctx)

	// Determine which template name to execute (same logic as before).
	execName := "content"
	if tc.clone.Lookup(execName) == nil {
		execName = filepath.Base(name) + ".html"
	}

	execErr := ctx.RenderTemplate(tc.clone, execName, data)

	tc.holder.clear()
	entry.pool.Put(tc)

	return execErr
}

func (v *ViewManager) loadTemplate(name string) (*templateEntry, error) {
	// If not in dev mode, try cache first.
	if !v.DevMode {
		v.mu.RLock()
		e, ok := v.cache[name]
		v.mu.RUnlock()
		if ok {
			return e, nil
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
	// that reference T can be parsed. The pool's New function will clone this
	// template and override T with a real holder-based implementation.
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

	// Build the entry with a pool whose New function creates a clone that has
	// a real, holder-backed T function. Clones are recycled between requests
	// so Clone() is called at most once per pool miss (typically once per
	// goroutine under sustained load) rather than once per request.
	entry := &templateEntry{tpl: parsed}
	entry.pool = sync.Pool{
		New: func() interface{} {
			holder := &tCtxHolder{}
			clone, cloneErr := parsed.Clone()
			if cloneErr != nil {
				// Allocation failure is fatal here; return nil and let
				// Render handle the nil case gracefully.
				return nil
			}
			clone = clone.Funcs(template.FuncMap{
				"T": holder.translate,
			})
			return &templateClone{clone: clone, holder: holder}
		},
	}

	if !v.DevMode {
		v.mu.Lock()
		v.cache[name] = entry
		v.mu.Unlock()
	}
	return entry, nil
}

// SetDefaultLayout sets the default layout file (relative to TemplateDir).
func (v *ViewManager) SetDefaultLayout(layout string) {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.DefaultLayout = layout
	// clear cache to ensure layout change takes effect
	v.cache = make(map[string]*templateEntry)
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
	v.cache = make(map[string]*templateEntry)
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
		v.cache = make(map[string]*templateEntry)
	}
	v.mu.Unlock()
}
