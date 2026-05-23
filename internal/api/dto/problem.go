package dto

// Problem mirrors the RFC 7807 error shape produced by httpx.WriteProblem.
// Defined here so external consumers can import the schema without
// pulling the internal httpx package.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code,omitempty"`
}
