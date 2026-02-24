package main

import (
	"fmt"
	"log"
	"net/http"

	mailer "github.com/undiegomejia/flow/contrib/plugin/mailer"
	"github.com/undiegomejia/flow/pkg/flow"
)

func main() {
	app := flow.New("mailer-demo")

	// Create a mock mailer for local demo. Swap this for SMTPAdapter in prod.
	m := mailer.NewMockMailer()
	if err := app.RegisterService("mailer", m); err != nil {
		log.Fatalf("failed to register mailer: %v", err)
	}

	app.SetRouter(http.NewServeMux())
	app.Handler() // ensure middleware wiring

	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// simple demo endpoint: send a test email to the configured recipient
		svc, ok := app.GetService("mailer")
		if !ok {
			http.Error(w, "mailer not configured", http.StatusInternalServerError)
			return
		}
		m, ok := svc.(mailer.Mailer)
		if !ok {
			http.Error(w, "invalid mailer type", http.StatusInternalServerError)
			return
		}
		// In a real app you'd validate inputs. Keep demo small.
		if err := m.Send("test@example.local", "Hello from Flow", "This is a demo."); err != nil {
			http.Error(w, fmt.Sprintf("send error: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "email queued (mock)")
	}))

	fmt.Println("starting demo on :8080")
	if err := http.ListenAndServe(":8080", app.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
