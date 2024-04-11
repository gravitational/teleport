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

package tpm

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tpm")

// serialString converts a serial number into a readable colon-delimited hex
// string thats user-readable e.g ab:ab:ab:ff:ff:ff
func serialString(serial *big.Int) string {
	hex := serial.Text(16)
	if len(hex)%2 == 1 {
		hex = "0" + hex
	}

	out := strings.Builder{}
	for i := 0; i < len(hex); i += 2 {
		if i != 0 {
			out.WriteString(":")
		}
		out.WriteString(hex[i : i+2])
	}
	return out.String()
}

// hashEKPub hashes the public part of an EK key. The key is first marshaled
// in PKIX format, hashed with SHA256, and returned as a hexadecimal string.
func hashEKPub(key crypto.PublicKey) (string, error) {
	marshaled, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", trace.Wrap(err)
	}
	hashed := sha256.Sum256(marshaled)
	return fmt.Sprintf("%x", hashed), nil
}

// QueryRes is the result of the TPM query performed by Query.
type QueryRes struct {
	// EKPub is the PKIX marshaled public part of the EK.
	EKPub []byte
	// EKPubHash is the SHA256 hash of the PKIX marshaled EKPub in hexadecimal
	// format.
	EKPubHash string
	// EKCertPresent is true if an EK cert is present in the TPM.
	EKCertPresent bool
	// EKCert is the ASN.1 DER encoded EK cert.
	EKCert []byte
	// EKCertSerial is the serial number of the EK cert represented as a colon
	// delimited hex string.
	EKCertSerial string
}

// Query returns information about the TPM on a system, including the
// EKPubHash and EKCertSerial which are needed to configure TPM joining.
func Query(ctx context.Context, log *slog.Logger) (*QueryRes, error) {
	ctx, span := tracer.Start(ctx, "Query")
	defer span.End()

	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			log.WarnContext(
				ctx,
				"Failed to close TPM",
				slog.String("error", err.Error()),
			)
		}
	}()
	return queryWithTPM(ctx, log, tpm)
}

func queryWithTPM(
	ctx context.Context, log *slog.Logger, tpm *attest.TPM,
) (*QueryRes, error) {
	ctx, span := tracer.Start(ctx, "queryWithTPM")
	defer span.End()

	data := &QueryRes{}

	eks, err := tpm.EKs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(eks) == 0 {
		// This is a pretty unusual case, `go-attestation` will attempt to
		// create an EK if no EK Certs are present in the NVRAM of the TPM.
		// Either way, it lets us catch this early in case `go-attestation`
		// misbehaves.
		return nil, trace.BadParameter(
			"no endorsement keys found in tpm",
		)
	}

	// The first EK returned by `go-attestation` will be an RSA based EK key or
	// EK cert. On Windows, ECC certs may also be returned following this. At
	// this time, we are only interested in RSA certs, so we just consider the
	// first thing returned.
	ekPub, err := x509.MarshalPKIXPublicKey(eks[0].Public)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data.EKPub = ekPub
	data.EKPubHash, err = hashEKPub(eks[0].Public)
	if err != nil {
		return nil, trace.Wrap(err, "hashing ekpub")
	}

	if eks[0].Certificate != nil {
		data.EKCert = eks[0].Certificate.Raw
		data.EKCertSerial = serialString(eks[0].Certificate.SerialNumber)
		data.EKCertPresent = true
	}
	log.DebugContext(ctx, "Successfully queried TPM", "data", data)
	return data, nil
}

// Attest provides the information necessary to perform a TPM join to a Teleport
// cluster. It returns a solve function which should be called when the
// encrypted credential challenge is received from the server.
// The Close function must be called if Attest returns in a non-error state.
func Attest(ctx context.Context, log *slog.Logger) (
	data *QueryRes,
	attestParams *attest.AttestationParameters,
	solve func(ec *attest.EncryptedCredential) ([]byte, error),
	close func() error,
	err error,
) {
	ctx, span := tracer.Start(ctx, "Attest")
	defer span.End()

	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, nil, nil, nil, trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			if err := tpm.Close(); err != nil {
				log.WarnContext(
					ctx,
					"Failed to close TPM",
					slog.String("error", err.Error()),
				)
			}
		}
	}()

	data, attestParams, solve, err = attestWithTPM(ctx, log, tpm)
	if err != nil {
		return nil, nil, nil, nil, trace.Wrap(err, "attesting with TPM")
	}

	return data, attestParams, solve, tpm.Close, nil

}

func attestWithTPM(ctx context.Context, log *slog.Logger, tpm *attest.TPM) (
	data *QueryRes,
	attestParams *attest.AttestationParameters,
	solve func(ec *attest.EncryptedCredential) ([]byte, error),
	err error,
) {
	ctx, span := tracer.Start(ctx, "attestWithTPM")
	defer span.End()

	queryData, err := queryWithTPM(ctx, log, tpm)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "querying TPM")
	}

	// Create AK and calculate attestation parameters.
	ak, err := tpm.NewAK(&attest.AKConfig{})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "creating ak")
	}
	log.DebugContext(ctx, "Successfully generated AK for TPM")
	attParams := ak.AttestationParameters()
	solve = func(ec *attest.EncryptedCredential) ([]byte, error) {
		log.DebugContext(ctx, "Solving credential challenge")
		return ak.ActivateCredential(tpm, *ec)
	}
	return queryData, &attParams, solve, nil
}
