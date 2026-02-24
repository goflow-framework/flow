package mailer

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"time"
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
	// If false, Send will open a plain TCP connection.
	UseTLS bool

	// Auth allows callers to provide a custom smtp.Auth implementation.
	// If nil and username is set, the adapter will fall back to smtp.PlainAuth.
	Auth smtp.Auth

	// AuthFactory is an optional factory used to create an smtp.Auth for a
	// given host. It is evaluated before the default PlainAuth fallback. Use
	// this to provide dynamic token-based auth (e.g., OAuth2/XOAUTH2).
	AuthFactory func(host string) smtp.Auth

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
	// InsecureSkipVerify controls whether the TLS client will verify the
	// server certificate. Useful for tests against self-signed certs. Do
	// NOT enable in production unless you understand the security risk.
	InsecureSkipVerify bool
}

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

// Send sends a minimal email. The adapter will attempt retries according to
// its configuration and supports both plain and explicit TLS transports.
func (s *SMTPAdapter) Send(to, subject, body string) error {
	if s == nil {
		return fmt.Errorf("smtp adapter: nil")
	}

	msg := "Subject: " + subject + "\r\n" + "\r\n" + body

	// prepare retry/backoff defaults
	attempts := 1
	if s.Retries > 0 {
		attempts = s.Retries + 1
	}
	if s.Backoff <= 0 {
		s.Backoff = 250 * time.Millisecond
	}
	if s.Timeout <= 0 {
		s.Timeout = 5 * time.Second
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := s.sendOnce(to, []byte(msg)); err != nil {
			lastErr = err
			if i+1 < attempts {
				time.Sleep(s.Backoff * time.Duration(i+1))
				continue
			}
			break
		}
		return nil
	}
	return lastErr
}

// sendOnce performs one SMTP send attempt using smtp.Client which allows
// timeout configuration and explicit TLS usage.
func (s *SMTPAdapter) sendOnce(to string, msg []byte) error {
	host := s.host()
	dialer := net.Dialer{Timeout: s.Timeout}

	var client *smtp.Client
	if s.UseTLS {
		// Dial and perform TLS handshake in one step.
		tlsConn, err := tls.DialWithDialer(&dialer, "tcp", s.addr, &tls.Config{ServerName: host, InsecureSkipVerify: s.InsecureSkipVerify})
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(tlsConn, host)
		if err != nil {
			_ = tlsConn.Close()
			return err
		}
		defer func() { _ = client.Quit(); _ = tlsConn.Close() }()
	} else {
		conn, err := dialer.Dial("tcp", s.addr)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, host)
		if err != nil {
			_ = conn.Close()
			return err
		}
		defer func() { _ = client.Quit(); _ = conn.Close() }()
	}

	// determine auth
	auth := s.Auth
	if auth == nil && s.AuthFactory != nil {
		auth = s.AuthFactory(host)
	}
	if auth == nil && s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, host)
	}
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
	}

	// from address: use username when available, otherwise use a sensible default
	from := s.username
	if from == "" {
		from = "noreply@" + host
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
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

// XOAUTH2 implements the smtp.Auth interface for XOAUTH2 style tokens.
// It constructs the initial client response as base64("user=...\1auth=Bearer <token>\1\1").
type XOAUTH2 struct {
	Username string
	Token    string
}

func NewXOAuth2Auth(username, token string) smtp.Auth {
	return &XOAUTH2{Username: username, Token: token}
}

func (a *XOAUTH2) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// XOAUTH2 is only meaningful over TLS or to servers that advertise it.
	// Build the initial client response.
	resp := "user=" + a.Username + "\x01auth=Bearer " + a.Token + "\x01\x01"
	return "XOAUTH2", []byte(base64.StdEncoding.EncodeToString([]byte(resp))), nil
}

func (a *XOAUTH2) Next(fromServer []byte, more bool) ([]byte, error) {
	// XOAUTH2 does not expect multi-step challenges from typical servers.
	return nil, nil
}
