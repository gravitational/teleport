//go:build tpmsimulator

package tpm

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"io"
	"log/slog"
	"math/big"
	"testing"

	"github.com/google/go-attestation/attest"
	tpmsimulator "github.com/google/go-tpm-tools/simulator"
	"github.com/google/go-tpm/legacy/tpm2"
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

	origEK, err := attestTPM.EKs()
	require.NoError(t, err)

	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)
	_, err = x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	fakeEKCert := &x509.Certificate{
		SerialNumber: big.NewInt(1337),
	}
	fakeEKBytes, err := x509.CreateCertificate(rand.Reader, fakeEKCert, ca, origEK[0].Public, caPrivKey)
	require.NoError(t, err)

	const nvramRSACertIndex = 0x1c00002

	sessionHandle := tpm2.HandlePasswordSession

	session := tpm2.AuthCommand{Session: sessionHandle, Attributes: tpm2.AttrContinueSession}

	err = tpm2.NVDefineSpace(
		sim,
		tpm2.HandleOwner,
		nvramRSACertIndex,
		"",
		"",
		nil,
		tpm2.AttrOwnerWrite|tpm2.AttrOwnerRead|tpm2.AttrReadSTClear,
		uint16(len(fakeEKBytes)),
	)
	require.NoError(t, err)

	err = tpm2.NVWrite(sim, tpm2.HandleOwner, nvramRSACertIndex, "", fakeEKBytes, 0)
	require.NoError(t, err)
	t.Run("query", func(t *testing.T) {
		res, err := query(ctx, slog.Default(), attestTPM)
		require.NoError(t, err)
		assert.NotEmpty(t, res.EKPub)
		assert.NotEmpty(t, res.EKCertSerial)
	})

	// TODO: Create fake EKCert and write it to the TPM!

}
