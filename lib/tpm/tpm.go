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
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
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

	out := strings.Builder{}
	// Handle odd-sized strings.
	if len(hex)%2 == 1 {
		out.WriteRune('0')
		out.WriteRune(rune(hex[0]))
		if len(hex) > 1 {
			out.WriteRune(':')
		}
		hex = hex[1:]
	}
	for i := 0; i < len(hex); i += 2 {
		if i != 0 {
			out.WriteString(":")
		}
		out.WriteString(hex[i : i+2])
	}
	return out.String()
}

// hashEKPub hashes the public part of an EK key. The key is hashed with SHA256,
// and returned as a hexadecimal string.
func hashEKPub(pkixPublicKey []byte) (string, error) {
	hashed := sha256.Sum256(pkixPublicKey)
	return fmt.Sprintf("%x", hashed), nil
}

// QueryRes is the result of the TPM query performed by Query.
type QueryRes struct {
	// EKPub is the PKIX marshaled public part of the EK.
	EKPub []byte
	// EKPubHash is the SHA256 hash of the PKIX marshaled EKPub in hexadecimal
	// format.
	EKPubHash string
	// EKCert holds the information about the EKCert if present. If nil, the
	// TPM does not have an EKCert.
	EKCert *QueryEKCert
}

// QueryEKCert contains the EKCert information if present.
type QueryEKCert struct {
	// Raw is the ASN.1 DER encoded EKCert.
	Raw []byte
	// SerialNumber is the serial number of the EKCert represented as a colon
	// delimited hex string.
	SerialNumber string
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
	return QueryWithTPM(ctx, log, tpm)
}

// QueryWithTPM is similar to Query, but accepts an already opened TPM.
func QueryWithTPM(
	ctx context.Context, log *slog.Logger, tpm *attest.TPM,
) (*QueryRes, error) {
	ctx, span := tracer.Start(ctx, "QueryWithTPM")
	defer span.End()

	data := &QueryRes{}

	eks, err := tpm.EKs()
	if err != nil {
		return nil, trace.Wrap(err, "querying EKs")
	}
	// Be a good citizen and check the slice bounds. This is not expected to
	// happen.
	if len(eks) == 0 {
		return nil, trace.BadParameter("no endorsement keys found in tpm")
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
	data.EKPubHash, err = hashEKPub(ekPub)
	if err != nil {
		return nil, trace.Wrap(err, "hashing ekpub")
	}

	if eks[0].Certificate != nil {
		data.EKCert = &QueryEKCert{
			Raw:          eks[0].Certificate.Raw,
			SerialNumber: serialString(eks[0].Certificate.SerialNumber),
		}
	}
	log.DebugContext(ctx, "Successfully queried TPM", "data", data)
	return data, nil
}

// Attestation holds the information necessary to perform a TPM join to a
// Teleport cluster.
type Attestation struct {
	// Data holds the queried information about the EK and EKCert if present.
	Data QueryRes
	// AttestParams holds the attestation parameters for the AK created for
	// this join ceremony.
	AttestParams attest.AttestationParameters
	// Solve is a function that should be called when the encrypted credential
	// challenge is received from the server.
	Solve func(ec *attest.EncryptedCredential) ([]byte, error)
}

// Attest provides the information necessary to perform a TPM join to a Teleport
// cluster. It returns a solve function which should be called when the
// encrypted credential challenge is received from the server.
// The Close function must be called if Attest returns in a non-error state.
func Attest(ctx context.Context, log *slog.Logger) (
	att *Attestation,
	close func() error,
	err error,
) {
	ctx, span := tracer.Start(ctx, "Attest")
	defer span.End()

	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
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

	att, err = AttestWithTPM(ctx, log, tpm)
	if err != nil {
		return nil, nil, trace.Wrap(err, "attesting with TPM")
	}

	return att, tpm.Close, nil

}

// AttestWithTPM is similar to Attest, but accepts an already opened TPM.
func AttestWithTPM(ctx context.Context, log *slog.Logger, tpm *attest.TPM) (
	att *Attestation,
	err error,
) {
	ctx, span := tracer.Start(ctx, "AttestWithTPM")
	defer span.End()

	queryData, err := QueryWithTPM(ctx, log, tpm)
	if err != nil {
		return nil, trace.Wrap(err, "querying TPM")
	}

	// Create AK and calculate attestation parameters.
	ak, err := tpm.NewAK(&attest.AKConfig{})
	if err != nil {
		return nil, trace.Wrap(err, "creating ak")
	}
	log.DebugContext(ctx, "Successfully generated AK for TPM")

	return &Attestation{
		Data:         *queryData,
		AttestParams: ak.AttestationParameters(),
		Solve: func(ec *attest.EncryptedCredential) ([]byte, error) {
			log.DebugContext(ctx, "Solving credential challenge")
			return ak.ActivateCredential(tpm, *ec)
		},
	}, nil
}

// PrintQuery prints a human-readable summary of the TPM information to the
// specified io.Writer.
func PrintQuery(data *QueryRes, debug bool, w io.Writer) {
	_, _ = fmt.Fprintf(w, "TPM Information\n")
	_, _ = fmt.Fprintf(w, "EK Public Hash: %s\n", data.EKPubHash)
	_, _ = fmt.Fprintf(w, "EK Certificate Detected: %t\n", data.EKCert != nil)
	if data.EKCert != nil {
		_, _ = fmt.Fprintf(w, "EK Certificate Serial: %s\n", data.EKCert.SerialNumber)
	}
	if debug {
		_, _ = fmt.Fprintf(w, "EK Public:\n%s", pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: data.EKPub,
		}))
		if data.EKCert != nil {
			_, _ = fmt.Fprintf(w, "EK Certificate:\n%s", pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: data.EKCert.Raw,
			}))
		}
	}
}
