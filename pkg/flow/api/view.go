package api

import (
	"html/template"
)

// ViewManager defines template rendering and FuncMap management used by
// controllers and middleware. Implementations must be safe for concurrent
// use and support toggling dev mode.
type ViewManager interface {
	Render(name string, data interface{}, ctx Context) error
	SetFuncMap(m template.FuncMap)
	SetDevMode(dev bool)
	SetDefaultLayout(layout string)
}
