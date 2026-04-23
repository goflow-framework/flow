package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/goflow-framework/flow/pkg/flow"
)

func main() {
	app := flow.New("view-demo",
		flow.WithViewsDefaultLayout("layouts/application.html"),
		flow.WithViewsFuncMap(template.FuncMap{
			"year":  func() string { return strconv.Itoa(time.Now().Year()) },
			"shout": func(s string) string { return strings.ToUpper(s) },
		}),
		flow.WithViewsDevMode(true),
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := flow.NewContext(app, w, r)
		_ = ctx.Render("greet/hello", map[string]string{"Name": "Alice"})
	})

	log.Println("listening on :3002")
	srv := &http.Server{Addr: ":3002", Handler: nil, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
