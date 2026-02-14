package flow

import (
	"errors"
	"testing"
)

func TestValidatePluginVersion(t *testing.T) {
	cases := []struct{
		in string
		wantErr bool
		expectErr error
	}{
		{"", true, ErrPluginVersionEmpty},
		{"not-a-version", true, ErrPluginVersionInvalid},
		{"v1.0.0", true, ErrPluginIncompatibleMajor}, // major != PluginAPIMajor (0)
		{"0.1.2", false, nil},
		{"v0.2", false, nil},
		{"0.2.3", false, nil},
	}

	for _, c := range cases {
		err := ValidatePluginVersion(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("ValidatePluginVersion(%q) wantErr=%v got err=%v", c.in, c.wantErr, err)
		}
		if c.expectErr != nil && err != nil {
			if !errors.Is(err, c.expectErr) {
				t.Fatalf("ValidatePluginVersion(%q) expected sentinel %v got %v", c.in, c.expectErr, err)
			}
		}
	}
}
