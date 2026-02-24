package flow

import (
    "bytes"
    "encoding/json"
    "testing"
)

func TestJSONLogger_RedactsFields(t *testing.T) {
    buf := &bytes.Buffer{}
    jl := NewJSONLogger(buf)
    jl.Log("info", "hello", map[string]interface{}{"api_key": "secretvalue"})

    var entry map[string]interface{}
    if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
        t.Fatalf("unmarshal log entry: %v", err)
    }
    fieldsI, ok := entry["fields"]
    if !ok {
        t.Fatalf("expected fields in log entry")
    }
    fields := fieldsI.(map[string]interface{})
    if got, ok := fields["api_key"]; !ok {
        t.Fatalf("expected api_key field present")
    } else if got != "[REDACTED]" {
        t.Fatalf("expected api_key redacted, got %v", got)
    }
}
