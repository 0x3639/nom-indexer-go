package indexer

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestGetVotingID_RoundTrip confirms that for a valid hex hash, getVotingID
// produces a stable 32-byte hash (does NOT return the original id unless the
// encode/decode chain fails).
func TestGetVotingID_RoundTrip(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}

	id := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	got := i.getVotingID(id)
	if got == "" {
		t.Fatal("getVotingID returned empty string")
	}
	if len(got) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars: %q", len(got), got)
	}
	// Determinism — same input must yield the same output.
	if got2 := i.getVotingID(id); got2 != got {
		t.Errorf("getVotingID non-deterministic: %q vs %q", got, got2)
	}
}

func TestGetVotingID_InvalidHashReturnsInput(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}
	bad := "not-a-hex-hash"
	if got := i.getVotingID(bad); got != bad {
		t.Errorf("expected pass-through of invalid hash, got %q", got)
	}
}

func TestGetFusionCancelID_RoundTrip(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}
	id := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	got := i.getFusionCancelID(id)
	if got == "" || len(got) != 64 {
		t.Errorf("expected 64-char hex hash, got %q", got)
	}
}

func TestGetFusionCancelID_InvalidHashReturnsInput(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}
	bad := "xx"
	if got := i.getFusionCancelID(bad); got != bad {
		t.Errorf("expected pass-through of invalid hash, got %q", got)
	}
}

func TestGetStakeCancelID_RoundTrip(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}
	id := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	got := i.getStakeCancelID(id)
	if got == "" || len(got) != 64 {
		t.Errorf("expected 64-char hex hash, got %q", got)
	}
}

func TestGetStakeCancelID_InvalidHashReturnsInput(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}
	bad := "xx"
	if got := i.getStakeCancelID(bad); got != bad {
		t.Errorf("expected pass-through of invalid hash, got %q", got)
	}
}

// TestGetPillarOwnerAddress confirms the read-lock path returns mapped owners
// and empty for unknown names.
func TestGetPillarOwnerAddress(t *testing.T) {
	i := &Indexer{
		logger:            zap.NewNop(),
		pillarNameToOwner: map[string]string{"alphanet-1": "z1qowner1"},
	}
	if got := i.getPillarOwnerAddress("alphanet-1"); got != "z1qowner1" {
		t.Errorf("expected mapped owner, got %q", got)
	}
	if got := i.getPillarOwnerAddress("missing"); got != "" {
		t.Errorf("expected empty for missing, got %q", got)
	}
}

// TestSignalSubscriptionRestart confirms repeat signals do not block on a
// full buffered channel (the SDK reconnection callback must never block).
func TestSignalSubscriptionRestart_NonBlocking(t *testing.T) {
	i := &Indexer{
		logger:       zap.NewNop(),
		restartSubCh: make(chan struct{}, 1),
	}
	// First signal fills the buffer.
	i.signalSubscriptionRestart()
	// Second signal must drop silently rather than block.
	done := make(chan struct{})
	go func() {
		i.signalSubscriptionRestart()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("signalSubscriptionRestart blocked when buffer was full")
	}
}
