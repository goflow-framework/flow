package mailer

import "testing"

func TestNewSMTPAdapter_NotNil(t *testing.T) {
    a := NewSMTPAdapter("smtp.example.com:25", "user", "pass")
    if a == nil {
        t.Fatalf("expected adapter, got nil")
    }
}
