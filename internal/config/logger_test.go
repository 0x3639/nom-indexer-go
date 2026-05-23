package config

import (
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestLoggingConfig_BuildLogger(t *testing.T) {
	tests := []struct {
		name      string
		cfg       LoggingConfig
		wantErr   bool
		wantLevel zapcore.Level
	}{
		{
			name:      "defaults to info+console",
			cfg:       LoggingConfig{},
			wantLevel: zapcore.InfoLevel,
		},
		{
			name:      "debug level json",
			cfg:       LoggingConfig{Level: "debug", Format: "json"},
			wantLevel: zapcore.DebugLevel,
		},
		{
			name:      "warn level console",
			cfg:       LoggingConfig{Level: "warn", Format: "console"},
			wantLevel: zapcore.WarnLevel,
		},
		{
			name:      "error level mixed case",
			cfg:       LoggingConfig{Level: "ERROR", Format: "JSON"},
			wantLevel: zapcore.ErrorLevel,
		},
		{
			name:    "invalid level returns error",
			cfg:     LoggingConfig{Level: "nope"},
			wantErr: true,
		},
		{
			name:    "invalid format returns error",
			cfg:     LoggingConfig{Format: "xml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := tt.cfg.BuildLogger()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildLogger: %v", err)
			}
			if logger == nil {
				t.Fatal("logger is nil")
			}
			// Confirm the level was applied by checking Core().Enabled.
			if !logger.Core().Enabled(tt.wantLevel) {
				t.Errorf("expected level %v to be enabled", tt.wantLevel)
			}
			// And one level below should be disabled (except for debug, where
			// there is no level below).
			if tt.wantLevel > zapcore.DebugLevel {
				lower := tt.wantLevel - 1
				if logger.Core().Enabled(lower) {
					t.Errorf("expected level %v to be disabled (one below %v)", lower, tt.wantLevel)
				}
			}
		})
	}
}

func TestLoggingConfig_BuildLogger_LevelErrorMessage(t *testing.T) {
	_, err := (&LoggingConfig{Level: "garbage"}).BuildLogger()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "garbage") {
		t.Errorf("expected error to mention the bad level, got %q", err.Error())
	}
}
