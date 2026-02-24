package mailer

import (
	"fmt"
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
}

// NewSMTPAdapter constructs an SMTPAdapter. addr should be in the form
// "host:port".
func NewSMTPAdapter(addr, username, password string) *SMTPAdapter {
	return &SMTPAdapter{addr: addr, username: username, password: password}
}

// Send sends a minimal email using net/smtp. This implementation is
// intentionally small; callers may provide richer MIME headers if needed.
func (s *SMTPAdapter) Send(to, subject, body string) error {
	if s == nil {
		return fmt.Errorf("smtp adapter: nil")
	}
	auth := smtp.PlainAuth("", s.username, s.password, s.host())
	msg := "Subject: " + subject + "\r\n" + "\r\n" + body
	return smtp.SendMail(s.addr, auth, s.username, []string{to}, []byte(msg))
}

func (s *SMTPAdapter) host() string {
	// split host:port and return host portion
	for i := 0; i < len(s.addr); i++ {
		if s.addr[i] == ':' {
			return s.addr[:i]
		}
	}
	return s.addr
}
