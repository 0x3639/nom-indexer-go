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

// HTLC hashLock/preimage are ABI `bytes` that formatArg hands back as raw byte
// strings, so the handler encodes them unconditionally via bytesToHex([]byte(s)).
// The load-bearing case: raw bytes that *look* like hex (e.g. the 8 ASCII bytes
// "deadbeef") must be encoded to their true byte hex, not passed through.
func TestBytesToHex_RawBytesThatLookLikeHex(t *testing.T) {
	// []byte("deadbeef") is 8 ASCII bytes 0x64 0x65 0x61 0x64 0x62 0x65 0x65 0x66.
	if got := bytesToHex([]byte("deadbeef")); got != "6465616462656566" {
		t.Errorf("bytesToHex([]byte(\"deadbeef\")) = %q, want 6465616462656566", got)
	}
	// A true 4-byte value still encodes to 8 hex chars.
	if got := bytesToHex([]byte{0xde, 0xad, 0xbe, 0xef}); got != "deadbeef" {
		t.Errorf("bytesToHex(0xdeadbeef) = %q, want deadbeef", got)
	}
}
