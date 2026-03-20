package log

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/gravitational/teleport/session/common/logutils"
)

//go:fix inline
const TraceLevel = logutils.TraceLevel

//go:fix inline
func StringerAttr(s fmt.Stringer) slog.LogValuer {
	return logutils.StringerAttr(s)
}

//go:fix inline
type SlogTextHandlerConfig = logutils.SlogTextHandlerConfig

//go:fix inline
func NewSlogTextHandler(w io.Writer, cfg logutils.SlogTextHandlerConfig) *logutils.SlogTextHandler {
	return logutils.NewSlogTextHandler(w, cfg)
}

//go:fix inline
func NewPackageLogger(args ...any) *slog.Logger {
	return logutils.NewPackageLogger(args...)
}
