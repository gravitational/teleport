package auth

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

func Test_ssoDiagContext_writeToBackend(t *testing.T) {
	diag := &ssoDiagContext{
		authKind:  types.KindSAML,
		requestID: "123",
		info:      types.SSODiagnosticInfo{},
	}

	callCount := 0

	diag.createSSODiagnosticInfo = func(ctx context.Context, authKind string, authRequestID string, info types.SSODiagnosticInfo) error {
		callCount++
		require.Truef(t, info.TestFlow, "createSSODiagnosticInfo must not be called if info.TestFlow is false.")
		require.Equal(t, diag.authKind, authKind)
		require.Equal(t, diag.requestID, authRequestID)
		require.Equal(t, diag.info, info)
		return nil
	}

	// with TestFlow: false, no call is made.
	diag.info.TestFlow = false
	diag.writeToBackend(context.Background())
	require.Equal(t, 0, callCount)

	// with TestFlow: true, a call is made.
	diag.info.TestFlow = true
	diag.writeToBackend(context.Background())
	require.Equal(t, 1, callCount)
}
