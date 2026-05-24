package dto

// Status is the JSON shape returned by GET /api/v1/status. It is a quick
// readiness summary derived entirely from the indexer's database — it
// does not hit the Zenon node. IndexerLagSeconds is computed as
// (now - LatestTimestamp) and grows when the indexer falls behind.
type Status struct {
	LatestHeight      uint64 `json:"latest_height"`
	LatestTimestamp   int64  `json:"latest_timestamp"`
	IndexerLagSeconds int64  `json:"indexer_lag_seconds"`
	Version           string `json:"version"`
}
