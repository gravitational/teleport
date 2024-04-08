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

package tpmjoin

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
)

// AttestationParametersToProto converts an attest.AttestationParameters to
// its protobuf representation.
func AttestationParametersToProto(in attest.AttestationParameters) *proto.TPMAttestationParameters {
	return &proto.TPMAttestationParameters{
		Public:            in.Public,
		CreateData:        in.CreateData,
		CreateAttestation: in.CreateAttestation,
		CreateSignature:   in.CreateSignature,
	}
}

// AttestationParametersFromProto extracts an attest.AttestationParameters from
// its protobuf representation.
func AttestationParametersFromProto(in *proto.TPMAttestationParameters) attest.AttestationParameters {
	if in == nil {
		return attest.AttestationParameters{}
	}
	return attest.AttestationParameters{
		Public:            in.Public,
		CreateData:        in.CreateData,
		CreateAttestation: in.CreateAttestation,
		CreateSignature:   in.CreateSignature,
	}
}

// EncryptedCredentialToProto converts an attest.EncryptedCredential to
// its protobuf representation.
func EncryptedCredentialToProto(in *attest.EncryptedCredential) *proto.TPMEncryptedCredential {
	if in == nil {
		return nil
	}
	return &proto.TPMEncryptedCredential{
		CredentialBlob: in.Credential,
		Secret:         in.Secret,
	}
}

// EncryptedCredentialFromProto extracts an attest.EncryptedCredential from
// its protobuf representation.
func EncryptedCredentialFromProto(in *proto.TPMEncryptedCredential) *attest.EncryptedCredential {
	if in == nil {
		return nil
	}
	return &attest.EncryptedCredential{
		Credential: in.CredentialBlob,
		Secret:     in.Secret,
	}
}

type QueryData struct {
	EKPub         []byte
	EKPubHash     string
	EKCertPresent bool
	EKCert        []byte
	EKCertSerial  string
}

func hashPub(key []byte) string {
	hashed := sha256.Sum256(key)
	return fmt.Sprintf("%x", hashed)
}

func query(tpm *attest.TPM) (*QueryData, error) {
	data := &QueryData{}

	eks, err := tpm.EKs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(eks) == 0 {
		// This is a pretty unusual case, `go-attestation` will attempt to
		// create an EK if no EK Certs are present in the NVRAM of the TPM.
		// Either way, it lets us catch this early in case `go-attestation`
		// misbehaves.
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
	data.EKPubHash = hashPub(ekPub)

	if eks[0].Certificate == nil {
		return data, nil
	}
	data.EKCert = eks[0].Certificate.Raw
	return data, nil
}

func Query(ctx context.Context, log *slog.Logger) (*QueryData, error) {
	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			log.ErrorContext(ctx, "Failed to close TPM", slog.Any("error", err))
		}
	}()
	return query(tpm)
}

func Attest(ctx context.Context, log *slog.Logger) (
	data *QueryData,
	attestParams *attest.AttestationParameters,
	solve func(ec *attest.EncryptedCredential) ([]byte, error),
	close func() error,
	err error,
) {
	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, nil, nil, nil, trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			if err := tpm.Close(); err != nil {
				log.ErrorContext(ctx, "Failed to close TPM", slog.Any("error", err))
			}
		}
	}()

	queryData, err := query(tpm)
	if err != nil {
		return nil, nil, nil, nil, trace.Wrap(err, "querying TPM")
	}

	// Create AK and calculate attestation parameters.
	ak, err := tpm.NewAK(&attest.AKConfig{})
	if err != nil {
		return nil, nil, nil, nil, trace.Wrap(err, "creating ak")
	}
	attParams := ak.AttestationParameters()
	solve = func(ec *attest.EncryptedCredential) ([]byte, error) {
		return ak.ActivateCredential(tpm, *ec)
	}
	close = func() error {
		return tpm.Close()
	}
	return queryData, &attParams, solve, close, nil
}

type ValidateParams struct {
	EKCert       []byte
	EKKey        []byte
	AttestParams attest.AttestationParameters
	Solve        func(ec *attest.EncryptedCredential) ([]byte, error)
	AllowedCAs   []string
}

type ValidatedTPM struct {
	EKPubHash    string
	EKCertSerial string
}

func parseEK(
	ctx context.Context, log *slog.Logger, params ValidateParams,
) (*x509.Certificate, crypto.PublicKey, error) {
	ekCertPresent := len(params.EKCert) > 0
	ekKeyPresent := len(params.EKKey) > 0
	switch {
	case ekCertPresent:
		ekCert, err := attest.ParseEKCertificate(params.EKCert)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		ekPub, err := x509.MarshalPKIXPublicKey(ekCert.PublicKey)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return ekCert, ekPub, nil
	case ekKeyPresent:
		ekPub, err := x509.ParsePKIXPublicKey(params.EKKey)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return nil, ekPub, nil
	default:
		return nil, nil, trace.BadParameter("either EK cert or EK key must be provided")
	}
}

func verifyEKCert(ctx context.Context, log *slog.Logger, allowedCAs []string, ekCert *x509.Certificate) error {
	if ekCert == nil {
		return trace.BadParameter("tpm did not provide an EKCert to validate against allowed CAs")
	}

	// Collect CAs into a pool to use for validation
	caPool := x509.NewCertPool()
	for _, caPEM := range allowedCAs {
		if !caPool.AppendCertsFromPEM([]byte(caPEM)) {
			return trace.BadParameter("invalid CA PEM")
		}
	}
	// Validate EKCert against CA pool
	_, err := ekCert.Verify(x509.VerifyOptions{
		Roots: caPool,
		KeyUsages: []x509.ExtKeyUsage{
			// Go's x509 Verification doesn't support the EK certificate
			// ExtKeyUsage (http://oid-info.com/get/2.23.133.8.1), so we
			// allow any.
			x509.ExtKeyUsageAny,
		},
	})
	if err != nil {
		return trace.BadParameter("presented EKCert failed verification: %v", err)
	}
	return nil
}

func Validate(
	ctx context.Context, log *slog.Logger, params ValidateParams,
) (*ValidatedTPM, error) {
	ekCert, ekPub, err := parseEK(ctx, log, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(params.AllowedCAs) > 0 {
		if err := verifyEKCert(ctx, log, params.AllowedCAs, ekCert); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	marshalledEKPub, err := x509.MarshalPKIXPublicKey(ekPub)
	if err != nil {
		return nil, trace.Wrap(err, "marshalling EK public key")
	}
	validated := &ValidatedTPM{
		EKPubHash: hashPub(marshalledEKPub),
	}
	if ekCert != nil {
		validated.EKCertSerial = SerialString(ekCert.SerialNumber)
	}

	activationParameters := attest.ActivationParameters{
		TPMVersion: attest.TPMVersion20,
		AK:         *params.AttestParams,
		EK:         ekPub,
	}
	// The generate method completes initial validation that provides the
	// following assurances:
	// - The attestation key is of a secure length
	// - The attestation key is marked as created within a TPM
	// - The attestation key is marked as restricted (e.g cannot be used to
	//   sign or decrypt external data)
	// When the returned challenge is solved by the TPM using ActivateCredential
	// the following additional assurance is given:
	// - The attestation key resides in the same TPM as the endorsement key
	solution, encryptedCredential, err := activationParameters.Generate()
	if err != nil {
		return nil, trace.Wrap(err, "generating credential activation challenge")
	}
	clientSolution, err := params.Solve(encryptedCredential)
	if err != nil {
		return nil, trace.Wrap(err, "asking client to perform credential activation")
	}
	if subtle.ConstantTimeCompare(clientSolution, solution) == 0 {
		return nil, trace.BadParameter("invalid credential activation solution")
	}

	return nil, nil
}

// SerialString converts a serial number into a readable colon-delimited hex
// string thats user-readable e.g ab:ab:ab:ff:ff:ff
func SerialString(serial *big.Int) string {
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
