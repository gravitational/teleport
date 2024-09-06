//go:build tpmsimulator

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tpm_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/google/go-attestation/attest"
	gocmp "github.com/google/go-cmp/cmp"
	tpmsimulator "github.com/google/go-tpm-tools/simulator"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/fixtures"
	"github.com/gravitational/teleport/lib/tpm"
)

// fakeCmdChannel is used to inject the TPM simulator into `go-attestation`'s
// TPM wrapper.
type fakeCmdChannel struct {
	io.ReadWriteCloser
}

// MeasurementLog implements CommandChannelTPM20.
func (cc *fakeCmdChannel) MeasurementLog() ([]byte, error) {
	// Nothing to do here - we don't use the measurement log for tpm joining.
	return nil, nil
}

func writeEKCertToTPM(t *testing.T, sim *tpmsimulator.Simulator, data []byte) {
	// As per TCG Credential Profile EK for TPM 2.0, 2.2.1.4, the RSA 2048
	// EK certificate is stored in the TPM's NV index 0x1c00002.
	const nvramRSAEKCertIndex = 0x1c00002
	err := tpm2.NVDefineSpace(
		sim,
		tpm2.HandlePlatform, // Using Platform Authorization.
		nvramRSAEKCertIndex,
		"", // As this is the simulator, there isn't a password for Platform Authorization.
		"", // We do not configure a password for this index. This allows it to be read using the NV index as the auth handle.
		nil,
		tpm2.AttrPPWrite| // Allows this NV index to be written with platform authorization.
			tpm2.AttrPPRead| // Allows this NV index to be read with platform authorization.
			tpm2.AttrPlatformCreate| // Marks this index as created by the Platform
			tpm2.AttrAuthRead, // Allows the nv index to be used as an auth handle to read itself.

		uint16(len(data)),
	)
	require.NoError(t, err)

	err = tpm2.NVWrite(
		sim,
		tpm2.HandlePlatform,
		nvramRSAEKCertIndex,
		"",
		data,
		0,
	)
	require.NoError(t, err)
}

// To include tests based on this simulator, use the `tpmsimulator` build tag.
// This requires openssl libraries to be installed on the machine and findable
// by the compiler. On macOS:
// brew install openssl
// export C_INCLUDE_PATH="$(brew --prefix openssl)/include"
// export LIBRARY_PATH="$(brew --prefix openssl)/lib"
// go test ./lib/tpm -run TestWithSimulator -tags tpmsimulator

func TestWithSimulator(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := slog.Default()

	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		BasicConstraintsValid: true,
		IsCA:                  true,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)
	caBytes, err := x509.CreateCertificate(
		rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey,
	)
	require.NoError(t, err)
	caPEM := pem.EncodeToMemory(
		&pem.Block{Type: "CERTIFICATE", Bytes: caBytes},
	)
	caPool := x509.NewCertPool()
	require.True(t, caPool.AppendCertsFromPEM(caPEM))
	wrongCAPool := x509.NewCertPool()
	require.True(t, wrongCAPool.AppendCertsFromPEM([]byte(fixtures.TLSCACertPEM)))

	sim, err := tpmsimulator.GetWithFixedSeedInsecure(0)
	require.NoError(t, err)
	// This is the EKPubHash that results from the EK generated with the seed 0.
	const wantEKPubHash = "1b5bbe2e96054f7bc34ebe7ba9a4a9eac5611c6879285ceff6094fa556af485c"

	attestTPM, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion:     attest.TPMVersion20,
		CommandChannel: &fakeCmdChannel{ReadWriteCloser: sim},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, attestTPM.Close(), "TPM simulator close errored")
	})

	tpmEKs, err := attestTPM.EKs()
	require.NoError(t, err)
	wantEKPub, err := x509.MarshalPKIXPublicKey(tpmEKs[0].Public)
	require.NoError(t, err)

	t.Run("Without EKCert", func(t *testing.T) {
		att, err := tpm.AttestWithTPM(ctx, log, attestTPM)
		require.NoError(t, err)

		// Check QueryRes looks right.
		assert.Empty(t, gocmp.Diff(tpm.QueryRes{
			EKPubHash: wantEKPubHash,
			EKPub:     wantEKPub,
		}, att.Data))

		t.Run("Success", func(t *testing.T) {
			validated, err := tpm.Validate(ctx, log, tpm.ValidateParams{
				EKKey:        att.Data.EKPub,
				AttestParams: att.AttestParams,
				Solve:        att.Solve,
			})
			require.NoError(t, err)
			assert.Empty(t, gocmp.Diff(&tpm.ValidatedTPM{
				EKPubHash:      wantEKPubHash,
				EKCertVerified: false,
			}, validated))
		})
		t.Run("Failure due to missing EKCert", func(t *testing.T) {
			_, err = tpm.Validate(ctx, log, tpm.ValidateParams{
				EKKey:        att.Data.EKPub,
				AttestParams: att.AttestParams,
				Solve:        att.Solve,
				AllowedCAs:   caPool,
			})
			assert.ErrorContains(t, err, "tpm did not provide an EKCert to validate against allowed CAs")
		})
	})

	// Write fake EKCert to the TPM
	const ekCertSerialNum = 1337133713371337
	const ekCertSerialHex = "04:c0:1d:b4:00:b0:c9"
	fakeEKCert := &x509.Certificate{
		SerialNumber: big.NewInt(ekCertSerialNum),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	fakeEKBytes, err := x509.CreateCertificate(
		rand.Reader, fakeEKCert, ca, tpmEKs[0].Public, caPrivKey,
	)
	require.NoError(t, err)
	writeEKCertToTPM(t, sim, fakeEKBytes)

	t.Run("With EKCert", func(t *testing.T) {
		att, err := tpm.AttestWithTPM(ctx, log, attestTPM)
		require.NoError(t, err)

		// Check queryRes looks right.
		assert.Empty(t, gocmp.Diff(tpm.QueryRes{
			EKPubHash: wantEKPubHash,
			EKPub:     wantEKPub,
			EKCert: &tpm.QueryEKCert{
				Raw:          fakeEKBytes,
				SerialNumber: ekCertSerialHex,
			},
		}, att.Data))

		t.Run("Success without CAs", func(t *testing.T) {
			validated, err := tpm.Validate(ctx, log, tpm.ValidateParams{
				EKKey:        att.Data.EKPub,
				EKCert:       att.Data.EKCert.Raw,
				AttestParams: att.AttestParams,
				Solve:        att.Solve,
			})
			require.NoError(t, err)
			assert.Empty(t, gocmp.Diff(&tpm.ValidatedTPM{
				EKPubHash:      wantEKPubHash,
				EKCertVerified: false,
				EKCertSerial:   ekCertSerialHex,
			}, validated))
		})
		t.Run("Success with CAs", func(t *testing.T) {
			validated, err := tpm.Validate(ctx, log, tpm.ValidateParams{
				EKKey:        att.Data.EKPub,
				EKCert:       att.Data.EKCert.Raw,
				AttestParams: att.AttestParams,
				Solve:        att.Solve,
				AllowedCAs:   caPool,
			})
			require.NoError(t, err)
			assert.Empty(t, gocmp.Diff(&tpm.ValidatedTPM{
				EKPubHash:      wantEKPubHash,
				EKCertVerified: true,
				EKCertSerial:   ekCertSerialHex,
			}, validated))
		})
		t.Run("Failure with wrong CA", func(t *testing.T) {
			_, err := tpm.Validate(ctx, log, tpm.ValidateParams{
				EKKey:        att.Data.EKPub,
				EKCert:       att.Data.EKCert.Raw,
				AttestParams: att.AttestParams,
				Solve:        att.Solve,
				// Some random CA that won't match the EKCert.
				AllowedCAs: wrongCAPool,
			})
			assert.ErrorContains(t, err, "certificate signed by unknown authority")
		})
	})
}
