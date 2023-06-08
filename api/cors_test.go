package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetHeaders(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	SetHeaders(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Access-Control-Allow-Origin header should be set to '*'")
	}

	if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("Access-Control-Allow-Credentials header should be set to 'true'")
	}

	if rr.Header().Get("Access-Control-Allow-Methods") != "POST, PATCH, GET, OPTIONS, PUT, DELETE" {
		t.Errorf("Access-Control-Allow-Methods header should be set to 'POST, PATCH, GET, OPTIONS, PUT, DELETE'")
	}

	if rr.Header().Get("Access-Control-Allow-Headers") != "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization" {
		t.Errorf("Access-Control-Allow-Headers header should be set to 'Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization'")
	}
}
