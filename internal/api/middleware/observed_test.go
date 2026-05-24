package middleware

import (
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newObservedCore returns a zapcore.Core + observed logs at debug+.
// Tiny shim so test files can stay focused.
func newObservedCore() (zapcore.Core, *observer.ObservedLogs) {
	return observer.New(zapcore.DebugLevel)
}
