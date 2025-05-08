/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package awsra

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"io"
	"math/big"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/aws-spiffe-workload-helper/vendoredaws"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
)

// CertificateGenerator is an interface that generates a certificate.
type CertificateGenerator interface {
	GenerateCertificate(req tlsca.CertificateRequest) ([]byte, error)
}

// GenerateCredentialsRequest is a request to generate AWS credentials using the ~/.aws/config `credential_process` schema.
type GenerateCredentialsRequest struct {
	// Clock is the clock used to calculate the expiration time of the credentials.
	Clock clockwork.Clock
	// TrustAnchorARN is the ARN of the AWS IAM Roles Anywhere Trust Anchor.
	TrustAnchorARN string
	// ProfileARN is the ARN of the AWS IAM Roles Anywhere Profile.
	ProfileARN string
	// RoleARN is the ARN of the AWS IAM Role to generate credentials.
	RoleARN string
	// SubjectCommonName is the common name to use in the certificate.
	SubjectCommonName string
	// Not After indicates the expiration time of the certificate.
	NotAfter time.Time
	// AcceptRoleSessionName indicates whether this Profile accepts a role session name.
	// Setting the role session name when the Profile does not accept it will result in an error.
	AcceptRoleSessionName bool
	// CertificateGenerator generates the certificate required for
	CertificateGenerator CertificateGenerator
}

// Credentials contains the AWS credentials.
type Credentials struct {
	// Version is the schema version.
	// Always 1.
	Version int `json:"Version"`
	// AccessKeyId is an AWS access key id.
	AccessKeyID string `json:"AccessKeyId"`
	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string `json:"SecretAccessKey"`
	// SessionToken is the the AWS session token for temporary credentials.
	SessionToken string `json:"SessionToken"`
	// Expiration is ISO8601 timestamp string when the credentials expire.
	Expiration string `json:"Expiration"`
	// SerialNumber is the serial number of the certificate which was created and exchanged to obtain AWS Credentials.
	// When using these credentials, CloudTrail will log the certificate's Subject Common Name, if the profile accepts it.
	// Otherwise, the serial number is logged.
	// This field is not part of the credential_process schema.
	SerialNumber *big.Int `json:"-"`
}

// AsCredentialProcessOutput returns the credentials as a JSON string as required
// by the credential_process schema.
func (c *Credentials) AsCredentialProcessOutput() (string, error) {
	bs, err := json.Marshal(c)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(bs), nil
}

// GenerateCredentials generates AWS IAM Roles Anywhere credentials for the Application (IAM Roles Anywhere Profile) and Role ARN.
func GenerateCredentials(ctx context.Context, req GenerateCredentialsRequest) (*Credentials, error) {
	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKeyECDSA, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("unexpected private key type %T", privateKey)
	}

	certPEMBytes, err := req.CertificateGenerator.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: privateKey.Public(),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		Subject: pkix.Name{
			CommonName: req.SubjectCommonName,
		},
		NotAfter: req.NotAfter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionDuration := int(req.Clock.Until(req.NotAfter).Seconds())

	x509Cert, _, err := keys.X509Certificate(certPEMBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert X.509 cert into credentials (calls rolesnaywhere:CreateSession)
	signer := &rolesAnywhereSigner{
		PrivateKey: privateKeyECDSA,
		PublicKey:  privateKey.Public(),
		X509Cert:   x509Cert,
	}

	// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html#authentication-task4
	const signAlgo = "AWS4-X509-ECDSA-SHA256"
	opts := &vendoredaws.CredentialsOpts{
		TrustAnchorArnStr: req.TrustAnchorARN,
		ProfileArnStr:     req.ProfileARN,
		RoleArn:           req.RoleARN,
		SessionDuration:   sessionDuration,
	}

	if req.AcceptRoleSessionName {
		// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-create-session.html#create-session-and-assume-role
		// If you provide a custom role session name in the CreateSession request but custom role session names are not accepted, you will receive an Access Denied error.
		opts.RoleSessionName = req.SubjectCommonName
	}

	credentialProcessOutput, err := vendoredaws.GenerateCredentials(opts, signer, signAlgo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Credentials{
		Version:         credentialProcessOutput.Version,
		AccessKeyID:     credentialProcessOutput.AccessKeyId,
		SecretAccessKey: credentialProcessOutput.SecretAccessKey,
		SessionToken:    credentialProcessOutput.SessionToken,
		Expiration:      credentialProcessOutput.Expiration,
		SerialNumber:    x509Cert.SerialNumber,
	}, nil
}

type rolesAnywhereSigner struct {
	PublicKey  crypto.PublicKey
	PrivateKey *ecdsa.PrivateKey
	X509Cert   *x509.Certificate
}

// Public returns the public key.
func (ras *rolesAnywhereSigner) Public() crypto.PublicKey {
	return ras.PublicKey
}

// Sign receives the digest and options, and returns the signature.
func (ras *rolesAnywhereSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	var hash []byte
	switch opts.HashFunc() {
	case crypto.SHA256:
		sum := sha256.Sum256(digest)
		hash = sum[:]
	case crypto.SHA384:
		sum := sha512.Sum384(digest)
		hash = sum[:]
	case crypto.SHA512:
		sum := sha512.Sum512(digest)
		hash = sum[:]
	default:
		return nil, trace.BadParameter("invalid HashFunc %+v", opts.HashFunc())
	}

	sig, err := ecdsa.SignASN1(rand, ras.PrivateKey, hash)
	if err != nil {
		return nil, trace.BadParameter("signing with ecdsa.SignASN1 failed %v", err)
	}

	return sig, nil
}

// Certificate returns the X.509 certificate.
func (ras *rolesAnywhereSigner) Certificate() (certificate *x509.Certificate, err error) {
	return ras.X509Cert, nil
}

// CertificateChain returns the certificate chain, in this case an empty slice.
func (ras *rolesAnywhereSigner) CertificateChain() (certificateChain []*x509.Certificate, err error) {
	return nil, nil
}

// Close does nothing, and is only used to satisfy the [vendoredaws.Signer] interface.
func (ras *rolesAnywhereSigner) Close() {}
