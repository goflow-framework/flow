package mailer

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// startSimpleSMTPServer runs a tiny SMTP-like server sufficient for our
// adapter's non-TLS path. It captures the DATA body and sends it to the
// returned channel. The returned shutdown func stops the listener.
func startSimpleSMTPServer(t *testing.T) (addr string, msgs chan string, shutdown func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	msgs = make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		// greeting
		if _, err := fmt.Fprint(conn, "220 localhost SimpleSMTP\r\n"); err != nil {
			return
		}
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(strings.ToUpper(line), "EHLO"):
				if _, err := fmt.Fprint(conn, "250-localhost greets\r\n250 OK\r\n"); err != nil {
					return
				}
			case strings.HasPrefix(strings.ToUpper(line), "HELO"):
				if _, err := fmt.Fprint(conn, "250 OK\r\n"); err != nil {
					return
				}
			case strings.HasPrefix(strings.ToUpper(line), "MAIL FROM:"):
				fmt.Fprint(conn, "250 OK\r\n")
			case strings.HasPrefix(strings.ToUpper(line), "RCPT TO:"):
				fmt.Fprint(conn, "250 OK\r\n")
			case strings.ToUpper(line) == "DATA":
				if _, err := fmt.Fprint(conn, "354 End data with <CR><LF>.<CR><LF>\r\n"); err != nil {
					return
				}
				var b strings.Builder
				for {
					l, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if l == ".\r\n" || l == ".\n" {
						break
					}
					b.WriteString(l)
				}
				msgs <- b.String()
				if _, err := fmt.Fprint(conn, "250 OK\r\n"); err != nil {
					return
				}
			case strings.ToUpper(line) == "QUIT":
				if _, err := fmt.Fprint(conn, "221 Bye\r\n"); err != nil {
					return
				}
				return
			default:
				// ignore
				if _, err := fmt.Fprint(conn, "250 OK\r\n"); err != nil {
					return
				}
			}
		}
	}()
	return ln.Addr().String(), msgs, func() {
		_ = ln.Close()
		// wait for goroutine to exit or timeout
		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func TestSMTPAdapter_IntegrationLocal(t *testing.T) {
	addr, msgs, shutdown := startSimpleSMTPServer(t)
	defer shutdown()

	// wait briefly for server to be ready
	time.Sleep(20 * time.Millisecond)

	a := NewSMTPAdapter(addr, "", "")
	a.Timeout = 2 * time.Second
	a.Retries = 1
	a.Backoff = 50 * time.Millisecond

	if err := a.Send("test@local", "Integration", "Hello local SMTP"); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	select {
	case m := <-msgs:
		if !strings.Contains(m, "Hello local SMTP") {
			t.Fatalf("message body missing, got: %q", m)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for server to receive message")
	}
}
