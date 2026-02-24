package mailer

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
)

// Mailer is a small interface for sending simple emails. Keep it tiny and
// easy to implement for testing.
type Mailer interface {
	Send(to, subject, body string) error
}

// SMTPAdapter sends mail using an SMTP server. This is a thin wrapper that
// keeps the example small. Production adapters may offer pooling, TLS
// configuration, and retries.
type SMTPAdapter struct {
	addr     string
	username string
	password string
	// UseTLS indicates whether to establish an explicit TLS connection.
	// If false, Send will use the plain smtp.SendMail which may negotiate
	// STARTTLS depending on the server.
	UseTLS bool
}

		// Auth allows callers to provide a custom smtp.Auth implementation.
		// If nil and username is set, the adapter will fall back to smtp.PlainAuth.
		Auth smtp.Auth

		// Timeout configures a dial timeout for network operations. If zero a
		// sensible default (5s) is used.
		Timeout time.Duration

		// Retries is the number of retry attempts (not including the first
		// attempt). For example, Retries=2 results in up to 3 total attempts.
		Retries int

		// Backoff is the base backoff duration between retries. Duration will
		// be multiplied by attempt number (simple linear backoff). If zero
		// a default of 250ms is used.
		Backoff time.Duration
// NewSMTPAdapter constructs an SMTPAdapter. addr should be in the form
// "host:port".
func NewSMTPAdapter(addr, username, password string) *SMTPAdapter {
	return &SMTPAdapter{addr: addr, username: username, password: password}
}

// NewSMTPAdapterWithTLS constructs an SMTPAdapter that will attempt to
// use an explicit TLS connection when sending mail.
func NewSMTPAdapterWithTLS(addr, username, password string, useTLS bool) *SMTPAdapter {
	return &SMTPAdapter{addr: addr, username: username, password: password, UseTLS: useTLS}
}

// Send sends a minimal email. If UseTLS is true the implementation will
// establish a TLS connection to the server and perform the SMTP dialog
// over that connection. This implementation keeps behavior explicit but
// intentionally small for the example.
func (s *SMTPAdapter) Send(to, subject, body string) error {
	if s == nil {
		return fmt.Errorf("smtp adapter: nil")
	}

	msg := "Subject: " + subject + "\r\n" + "\r\n" + body

	if !s.UseTLS {
		auth := smtp.PlainAuth("", s.username, s.password, s.host())
		return smtp.SendMail(s.addr, auth, s.username, []string{to}, []byte(msg))
	}

	// Establish TLS connection and use smtp.Client for explicit TLS
	host := s.host()
	conn, err := tls.Dial("tcp", s.addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Quit(); _ = conn.Close() }()

	// If username is set, attempt AUTH
	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, host)
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}

	if err := c.Mail(s.username); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return c.Quit()
}

func (s *SMTPAdapter) host() string {
	// split host:port and return host portion
	if s == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(s.addr)
	if err != nil {
		return s.addr
	}
	return host
}
