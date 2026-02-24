package main

import (
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"

	mailer "github.com/undiegomejia/flow/contrib/plugin/mailer"
	"github.com/undiegomejia/flow/pkg/flow"
)

func main() {
	app := flow.New("mailer-xoauth2-demo")

	// Example AuthFactory that creates XOAUTH2 auth using a token from env.
	authFactory := func(host string) smtp.Auth {
		// Token could be refreshed here as needed.
		token := os.Getenv("XOAUTH2_TOKEN")
		if token == "" {
			return nil
		}
		return mailer.NewXOAuth2Auth("user@example.com", token)
	}

	// Create adapter with explicit TLS and an AuthFactory.
	a := mailer.NewSMTPAdapterWithTLS("smtp.example.com:587", "user@example.com", "", true)
	a.AuthFactory = authFactory

	if err := app.RegisterService("mailer", a); err != nil {
		log.Fatalf("register service: %v", err)
	}

	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svc, _ := app.GetService("mailer")
		m := svc.(mailer.Mailer)
		if err := m.Send("alice@example.com", "Hello", "XOAUTH2 demo"); err != nil {
			http.Error(w, fmt.Sprintf("send failed: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "sent (or attempted)")
	}))

	log.Println("starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", app.Handler()))
}
