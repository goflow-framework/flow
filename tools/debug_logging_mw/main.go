package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/goflow-framework/flow/pkg/flow"
)

func main() {
	var buf bytes.Buffer
	jl := flow.NewJSONLogger(&buf)

	// runtime check whether jl implements StructuredLogger
	if _, ok := interface{}(jl).(flow.StructuredLogger); ok {
		fmt.Println("jl DOES implement flow.StructuredLogger")
	} else {
		fmt.Println("jl does NOT implement flow.StructuredLogger")
	}

	mw := flow.LoggingMiddlewareWithRedaction(jl)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test-path", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	out := buf.String()
	fmt.Println("Raw buffer output:\n", out)

	// parse lines
	for i, line := range bytes.Split([]byte(out), []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			fmt.Printf("line %d not json: %s\n", i, string(line))
			continue
		}
		fmt.Printf("parsed entry %d: level=%v, msg=%v, fields=%v\n", i, entry["level"], entry["msg"], entry["fields"])
	}
}
