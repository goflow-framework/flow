package flow

import (
	"testing"

	"github.com/google/uuid"
)

// Benchmark the fastRequestID generator we added versus the standard uuid.New().String().
// This is a microbenchmark to show relative CPU and allocation cost in isolation.
func BenchmarkFastRequestID(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fastRequestID()
	}
}

func BenchmarkUUIDNew(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = uuid.New().String()
	}
}

func TestFastRequestIDFormat(t *testing.T) {
	id := fastRequestID()
	if id == "" {
		t.Fatal("fastRequestID returned empty string")
	}
	// basic sanity: should include a dash separator and be reasonably short
	found := false
	for i := 0; i < len(id); i++ {
		if id[i] == '-' {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("fastRequestID has unexpected format: %q", id)
	}
}
