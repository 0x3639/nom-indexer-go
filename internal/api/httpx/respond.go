package httpx

import (
	"encoding/json"
	"net/http"
)

// Problem is the RFC 7807 problem-details envelope. Fields that are unset
// are omitted from the JSON to keep responses compact.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code,omitempty"`
}

// WriteJSON marshals v and writes it with the given status code as
// application/json. On marshal failure it falls back to a 500 problem.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	body, err := json.Marshal(v)
	if err != nil {
		WriteProblem(w, http.StatusInternalServerError, "internal_error",
			"failed to encode response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteProblem writes an RFC 7807 problem with the given HTTP status,
// stable application code, and human-readable detail. Title is derived
// from the status code via http.StatusText.
func WriteProblem(w http.ResponseWriter, status int, code, detail string) {
	p := Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
		Code:   code,
	}
	body, err := json.Marshal(p)
	if err != nil {
		// Should be unreachable — Problem has only primitive fields.
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
		return
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
