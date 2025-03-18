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
	"io"
	"time"

	"github.com/aws/rolesanywhere-credential-helper/aws_signing_helper"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
)

type CertificateGenerator interface {
	GenerateCertificate(req tlsca.CertificateRequest) ([]byte, error)
}

type GenerateAWSRACredentialsRequest struct {
	Clock                clockwork.Clock
	TrustAnchorARN       string
	ProfileARN           string
	RoleARN              string
	SubjectCommonName    string
	NotAfter             time.Time
	CertificateGenerator CertificateGenerator
}

type GenerateAWSRACredentialsResponse struct {
	// Version is the schema version.
	Version int `json:"Version"`
	// AccessKeyId is an AWS access key id.
	AccessKeyID string `json:"AccessKeyId"`
	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string `json:"SecretAccessKey"`
	// SessionToken is the the AWS session token for temporary credentials.
	SessionToken string `json:"SessionToken"`
	// Expiration is ISO8601 timestamp when the credentials expire.
	Expiration string `json:"Expiration"`
}

// GenerateAWSRACredentials generates AWS IAM Roles Anywhere credentials for the Application (IAM Roles Anywhere Profile) and Role ARN.
func GenerateAWSRACredentials(ctx context.Context, req GenerateAWSRACredentialsRequest) (*GenerateAWSRACredentialsResponse, error) {
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
	opts := &aws_signing_helper.CredentialsOpts{
		TrustAnchorArnStr: req.TrustAnchorARN,
		ProfileArnStr:     req.ProfileARN,
		RoleArn:           req.RoleARN,
		SessionDuration:   sessionDuration,
	}

	signer := &rolesAnywhereSigner{
		PrivateKey: privateKeyECDSA,
		PublicKey:  privateKey.Public(),
		X509Cert:   x509Cert,
	}

	// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html#authentication-task4
	const signAlgo = "AWS4-X509-ECDSA-SHA256"

	output, err := aws_signing_helper.GenerateCredentials(opts, signer, signAlgo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &GenerateAWSRACredentialsResponse{
		Version:         1,
		AccessKeyID:     output.AccessKeyId,
		SecretAccessKey: output.SecretAccessKey,
		SessionToken:    output.SessionToken,
		Expiration:      output.Expiration,
	}, nil
}

type rolesAnywhereSigner struct {
	PublicKey  crypto.PublicKey
	PrivateKey *ecdsa.PrivateKey
	X509Cert   *x509.Certificate
}

func (ras *rolesAnywhereSigner) Public() crypto.PublicKey {
	return ras.PublicKey
}
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

func (ras *rolesAnywhereSigner) Certificate() (certificate *x509.Certificate, err error) {
	return ras.X509Cert, nil
}

func (ras *rolesAnywhereSigner) CertificateChain() (certificateChain []*x509.Certificate, err error) {
	return nil, nil
}

func (ras *rolesAnywhereSigner) Close() {}
