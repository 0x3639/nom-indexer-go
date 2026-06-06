package indexer

import "testing"

// bytesToHex is the small helper the HTLC handler uses to store hashLock/preimage.
func TestBytesToHex(t *testing.T) {
	got := bytesToHex([]byte{0xde, 0xad, 0xbe, 0xef})
	if got != "deadbeef" {
		t.Errorf("bytesToHex = %q, want deadbeef", got)
	}
	if bytesToHex(nil) != "" {
		t.Errorf("bytesToHex(nil) should be empty")
	}
}
