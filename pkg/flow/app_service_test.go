package flow

import "testing"

func TestApp_RegisterAndGetService(t *testing.T) {
	app := New("svc-test")
	if err := app.RegisterService("mailer", "smtp://localhost"); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}
	v, ok := app.GetService("mailer")
	if !ok {
		t.Fatalf("expected service to be present")
	}
	if s, _ := v.(string); s == "" {
		t.Fatalf("unexpected service value: %v", v)
	}
}
