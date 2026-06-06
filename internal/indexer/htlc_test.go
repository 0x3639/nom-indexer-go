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

// hexMaybe normalizes an ABI-decoded bytes value into lowercase hex: already-hex
// strings are lowercased, raw byte strings are hex-encoded, "" stays "".
func TestHexMaybe(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"already hex mixed case", "DEADbeef", "deadbeef"},
		{"empty", "", ""},
		{"raw bytes with high byte", "\xff\x01", "ff01"},
		{"non-hex char", "zz", "7a7a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hexMaybe(tc.in); got != tc.want {
				t.Errorf("hexMaybe(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
