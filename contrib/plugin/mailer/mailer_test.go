package mailer

import "testing"

func TestMockMailerRecordsMessage(t *testing.T) {
	mm := NewMockMailer()
	if mm == nil {
		t.Fatal("expected non-nil mock mailer")
	}
	if err := mm.Send("alice@example.com", "hi", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mm.Sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mm.Sent))
	}
	m := mm.Sent[0]
	if m.To != "alice@example.com" || m.Subject != "hi" || m.Body != "hello" {
		t.Fatalf("unexpected recorded message: %#v", m)
	}
}

func TestNewSMTPAdapterWithTLSFlag(t *testing.T) {
	s := NewSMTPAdapterWithTLS("smtp.example.com:587", "u", "p", true)
	if s == nil {
		t.Fatal("expected adapter")
	}
	if !s.UseTLS {
		t.Fatalf("expected UseTLS=true")
	}
}

func TestNewSMTPAdapter_NotNil(t *testing.T) {
	a := NewSMTPAdapter("smtp.example.com:25", "user", "pass")
	if a == nil {
		t.Fatalf("expected adapter, got nil")
	}
}
