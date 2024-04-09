//go:build tpmsimulator

package tpm

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/go-attestation/attest"
	tpmsimulator "github.com/google/go-tpm-tools/simulator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// To include tests based on this simulator, use the `tpmsimulator` build tag.
// This requires openssl libraries to be installed on the machine and findable
// by the compiler. On macOS:
// brew install openssl
// export C_INCLUDE_PATH="$(brew --prefix openssl)/include"
// export LIBRARY_PATH="$(brew --prefix openssl)/lib"
// go test ./e/lib/devicetrust/devicetrustv1 -run TestService_EnrollDevice -tags tpmsimulator

// fakeCmdChannel is used to inject the TPM simulator into `go-attestation`'s
// TPM wrapper.
type fakeCmdChannel struct {
	io.ReadWriteCloser
}

// MeasurementLog implements CommandChannelTPM20.
func (cc *fakeCmdChannel) MeasurementLog() ([]byte, error) {
	// Return nil, we inject an event log in handleEnrollStream
	return nil, nil
}

func TestWithSimulator(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sim, err := tpmsimulator.GetWithFixedSeedInsecure(0)
	require.NoError(t, err)

	attestTPM, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion:     attest.TPMVersion20,
		CommandChannel: &fakeCmdChannel{ReadWriteCloser: sim},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, attestTPM.Close())
	})

	t.Run("query", func(t *testing.T) {
		res, err := query(ctx, slog.Default(), attestTPM)
		require.NoError(t, err)
		assert.NotEmpty(t, res.EKPub)
	})
}
