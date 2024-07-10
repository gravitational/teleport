package internal

import (
	"log/slog"

	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
)

// AssertParams holds creation parameters for assert.Ceremony.
//
// Declared in internal so it can't be created by packages outside of
// devicetrustv1.
type AssertParams struct {
	Logger  *slog.Logger
	Storage *storage.S
}
