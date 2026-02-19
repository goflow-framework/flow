package flow

import (
    "errors"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestHTTPError_Error(t *testing.T) {
    base := errors.New("underlying")
    he := &HTTPError{StatusCode: 400, Code: "bad_request", Message: "bad", Err: base}
    if !strings.Contains(he.Error(), "bad") || !strings.Contains(he.Error(), "underlying") {
        t.Fatalf("unexpected Error() output: %q", he.Error())
    }
}

func TestDefaultErrorHandler_NonVerbose(t *testing.T) {
    rec := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    err := errors.New("boom internal")
    DefaultErrorHandler(rec, req, err, false)
    if rec.Code != http.StatusInternalServerError {
        t.Fatalf("expected 500 got %d", rec.Code)
    }
    if strings.Contains(rec.Body.String(), "boom internal") {
        t.Fatalf("did not expect internal message in non-verbose mode")
    }
}

func TestDefaultErrorHandler_VerboseHTTPError(t *testing.T) {
    rec := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    he := &HTTPError{StatusCode: 418, Code: "teapot", Message: "short", Err: errors.New("details")}
    DefaultErrorHandler(rec, req, he, true)
    if rec.Code != 418 {
        t.Fatalf("expected 418 got %d", rec.Code)
    }
    if !strings.Contains(rec.Body.String(), "details") {
        t.Fatalf("expected underlying details in verbose mode: %q", rec.Body.String())
    }
}
