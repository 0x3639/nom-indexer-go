package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_HappyPath(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSON(rr, 200, map[string]string{"hello": "world"})

	if got := rr.Code; got != 200 {
		t.Errorf("status = %d, want 200", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content-type = %q, want application/json", got)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["hello"] != "world" {
		t.Errorf("body = %v, want hello=world", body)
	}
}

func TestWriteJSON_MarshalFailureFallsBackToProblem(t *testing.T) {
	rr := httptest.NewRecorder()
	// channels are not JSON-marshalable
	WriteJSON(rr, 200, make(chan int))

	if rr.Code != 500 {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("content-type = %q, want application/problem+json", got)
	}
}

func TestWriteProblem_Shape(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteProblem(rr, 401, "unauthorized", "missing bearer token")

	if rr.Code != 401 {
		t.Errorf("status = %d, want 401", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("content-type = %q, want application/problem+json", got)
	}

	var p Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if p.Status != 401 || p.Title != "Unauthorized" || p.Code != "unauthorized" || p.Detail != "missing bearer token" {
		t.Errorf("problem = %+v", p)
	}
	if p.Type != "about:blank" {
		t.Errorf("type = %q, want about:blank", p.Type)
	}
}
