package logtest

import (
	"log/slog"

	"github.com/gravitational/teleport/session/common/logutils/logtest"
)

//go:fix inline
func InitLogger(verbose func() bool) {
	logtest.InitLogger(verbose)
}

//go:fix inline
func With(args ...any) *slog.Logger {
	return logtest.With(args...)
}

//go:fix inline
func NewLogger() *slog.Logger {
	return logtest.NewLogger()
}
