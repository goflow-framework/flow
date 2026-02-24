package mailer

import (
	"io"
	"net"
	"testing"
	"time"

	smtpserver "github.com/emersion/go-smtp"
)

// Backend and Session implementations for go-smtp to capture messages in tests.
type testBackend struct {
	msgs chan string
}

func (b *testBackend) Login(state *smtpserver.ConnectionState, username, password string) (smtpserver.Session, error) {
	return &testSession{msgs: b.msgs}, nil
}

func (b *testBackend) AnonymousLogin(state *smtpserver.ConnectionState) (smtpserver.Session, error) {
	return &testSession{msgs: b.msgs}, nil
}

type testSession struct {
	msgs chan string
}

func (s *testSession) Mail(from string, opts smtpserver.MailOptions) error { return nil }
func (s *testSession) Rcpt(to string) error                                { return nil }
func (s *testSession) Data(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.msgs <- string(b)
	return nil
}
func (s *testSession) Reset()        {}
func (s *testSession) Logout() error { return nil }

func startTestSMTPServer(t *testing.T) (addr string, shutdown func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	msgs := make(chan string, 1)
	be := &testBackend{msgs: msgs}
	s := smtpserver.NewServer(be)
	s.AllowInsecureAuth = true
	go func() {
		_ = s.Serve(ln)
	}()
	return ln.Addr().String(), func() {
		_ = s.Close()
		_ = ln.Close()
		close(msgs)
	}
}

func TestSMTPAdapter_IntegrationLocal(t *testing.T) {
	addr, shutdown := startTestSMTPServer(t)
	defer shutdown()

	// wait a short moment for server to be ready
	time.Sleep(50 * time.Millisecond)

	// use adapter with no auth for local test server
	a := NewSMTPAdapter(addr, "", "")
	// small timeouts to keep test fast
	a.Timeout = 2 * time.Second
	a.Retries = 1
	a.Backoff = 100 * time.Millisecond

	if err := a.Send("test@local", "Integration", "Hello local SMTP"); err != nil {
		t.Fatalf("send failed: %v", err)
	}
}
