package indexer

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestSafeBigIntToInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    *big.Int
		want     int64
		wantWarn bool
	}{
		{"nil", nil, 0, false},
		{"zero", big.NewInt(0), 0, false},
		{"positive small", big.NewInt(12345), 12345, false},
		{"negative small", big.NewInt(-12345), -12345, false},
		{"max int64", big.NewInt(math.MaxInt64), math.MaxInt64, false},
		{"min int64", big.NewInt(math.MinInt64), math.MinInt64, false},
		{"overflow positive", new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(1)), math.MaxInt64, true},
		{"overflow huge", new(big.Int).Exp(big.NewInt(2), big.NewInt(100), nil), math.MaxInt64, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			logger := zap.New(core)

			got := safeBigIntToInt64(tt.input, logger, "test overflow", zap.String("k", "v"))
			if got != tt.want {
				t.Errorf("safeBigIntToInt64(%v) = %d, want %d", tt.input, got, tt.want)
			}

			gotWarn := logs.Len() > 0
			if gotWarn != tt.wantWarn {
				t.Errorf("warn emitted = %v, want %v (logs=%v)", gotWarn, tt.wantWarn, logs.All())
			}
		})
	}
}

func TestQueueAccountBlockNotifyOmitsLargeInputsButQueues(t *testing.T) {
	var batch pgx.Batch
	ab := &models.AccountBlock{
		Hash:              "h1",
		MomentumHash:      "mh1",
		MomentumTimestamp: 123,
		MomentumHeight:    456,
		BlockType:         2,
		Height:            7,
		Address:           "z1qsender",
		ToAddress:         "z1qrecipient",
		Amount:            42,
		TokenStandard:     models.ZnnTokenStandard,
		Data:              strings.Repeat("d", 200),
	}
	txData := &models.TxData{
		Method: "BigCall",
		Inputs: map[string]string{
			"payload": strings.Repeat("x", 9000),
		},
	}

	if err := queueAccountBlockNotify(&batch, ab, txData); err != nil {
		t.Fatalf("queueAccountBlockNotify: %v", err)
	}
	if batch.Len() != 1 {
		t.Fatalf("batch.Len() = %d, want 1", batch.Len())
	}
}
