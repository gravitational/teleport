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

// https://github.com/mrcdb/tpm2_ek_cert_generator/blob/master/generate_ek_cert.sh

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
	err = tpm2.NVDefineSpace(
		sim,
		tpm2.HandlePlatform, // We act as the platform when creating this index.
		nvramRSACertIndex,
		"", // As this is the simulator, there isn't a password on the platform authorization.
		"", // We do not configure a password for this index. This allows it to be read using the NV index as the auth handle.
		nil,
		tpm2.AttrPPWrite| // Allows this NV index to be written with platform authorization.
			tpm2.AttrPPRead| // Allows this NV index to be read with platform authorization.
			tpm2.AttrPlatformCreate| // Marks this index as created by the Platform
			tpm2.AttrAuthRead, // Allows the nv index to be used as an auth handle to read itself.

		uint16(len(fakeEKBytes)),
	)
	require.NoError(t, err)

	err = tpm2.NVWrite(
		sim,
		tpm2.HandlePlatform,
		nvramRSACertIndex,
		"",
		fakeEKBytes,
		0,
	)
	require.NoError(t, err)

	read, err := tpm2.NVReadEx(
		sim,
		nvramRSACertIndex, // The index we want to read.
		nvramRSACertIndex, // Weird case: we act as the index itself when reading from it.
		"",                // The password for the NV index auth handle - left empty in NVDefineSpace
		0,
	)
	require.NoError(t, err)
	require.NotEmpty(t, read)

	t.Run("query", func(t *testing.T) {
		res, err := query(ctx, slog.Default(), attestTPM)
		require.NoError(t, err)
		assert.NotEmpty(t, res.EKPub)
		assert.NotEmpty(t, res.EKCertSerial)
	})

	// TODO: Create fake EKCert and write it to the TPM!

}
