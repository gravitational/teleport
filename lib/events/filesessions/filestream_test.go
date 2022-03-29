package filesessions

import (
	"context"
	"os"
	"testing"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCreateUpload(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		mode      constants.AuditMode
		err       error
		mkdirFunc utils.MkdirAllFunc
	}{
		"StrictModeSuccess": {
			mode: constants.AuditModeStrict,
		},
		"BestEffortModeSuccess": {
			mode: constants.AuditModeBestEffort,
		},
		"StrictModeFailure": {
			mode: constants.AuditModeStrict,
			err:  trace.AccessDenied(""),
			mkdirFunc: func(_ string, _ os.FileMode) error {
				// return non-nil error.
				return trace.AccessDenied("")
			},
		},
		"BestEffortModeFailure": {
			mode: constants.AuditModeBestEffort,
			mkdirFunc: func(_ string, _ os.FileMode) error {
				// return non-nil error.
				return trace.AccessDenied("")
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			handler, err := NewHandler(Config{
				Directory:    t.TempDir(),
				AuditMode:    test.mode,
				MkdirAllFunc: test.mkdirFunc,
			})
			require.NoError(t, err)

			_, err = handler.CreateUpload(ctx, session.ID(uuid.New().String()))
			if err != nil {
				require.Error(t, err)
				require.ErrorIs(t, test.err, err)
				return
			}

			require.NoError(t, err)
		})
	}
}
