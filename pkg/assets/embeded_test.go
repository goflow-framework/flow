package assets

import (
    "testing"
)

// Basic compile-time test to ensure the embedded dist placeholder exists.
func TestKeepFileExists(t *testing.T) {
    f := Assets()
    if _, err := f.Open(".keep"); err != nil {
        t.Fatalf("expected .keep to be present in embedded dist: %v", err)
    }
}
