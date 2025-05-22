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
	"crypto/x509"
	"crypto/x509/pkix"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	// AWSCredentialsSourceRolesAnywhere is the source of the credentials.
	// There's no official constant in AWS docs.
	AWSCredentialsSourceRolesAnywhere = "RolesAnywhere"
)

// CertificateGenerator is an interface that generates a certificate.
type CertificateGenerator interface {
	GenerateCertificate(req tlsca.CertificateRequest) ([]byte, error)
}

// GenerateCredentialsRequest is a request to generate AWS credentials using the AWS IAM Roles Anywhere integration.
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
	// DurationSeconds is the duration of the session.
	// If nil, the default duration of the Profile will be used.
	DurationSeconds *int
	// AcceptRoleSessionName indicates whether this Profile accepts a role session name.
	// Setting the role session name when the Profile does not accept it will result in an error.
	AcceptRoleSessionName bool
	// KeyStoreManager grants access to the AWS Roles Anywhere signer.
	KeyStoreManager KeyStoreManager
	/// Cache is used to get the current cluster name and cert authority keys.
	Cache Cache

	// CreateSession is the API used to create a session with AWS IAM Roles Anywhere.
	// This is used to mock the CreateSession API in tests.
	CreateSession func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error)
}

// KeyStoreManager defines methods to get signers using the server's keystore.
type KeyStoreManager interface {
	// GetTLSCertAndSigner selects a usable TLS keypair from the given CA
	// and returns the PEM-encoded TLS certificate and a [crypto.Signer].
	GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error)
}

// Cache is the subset of the cached resources that the Service queries.
type Cache interface {
	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	// GetClusterName returns the current cluster name.
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

func (r *GenerateCredentialsRequest) checkAndSetDefaults() error {
	if r.TrustAnchorARN == "" {
		return trace.BadParameter("trust anchor ARN is required")
	}
	if r.ProfileARN == "" {
		return trace.BadParameter("profile ARN is required")
	}
	if r.RoleARN == "" {
		return trace.BadParameter("role ARN is required")
	}
	if r.SubjectCommonName == "" {
		return trace.BadParameter("subject common name is required")
	}
	if r.KeyStoreManager == nil {
		return trace.BadParameter("certificate generator is required")
	}
	if r.KeyStoreManager == nil {
		return trace.BadParameter("certificate generator is required")
	}
	if r.Cache == nil {
		return trace.BadParameter("backend cache is required")
	}

	if r.CreateSession == nil {
		// Use the default CreateSession API.
		r.CreateSession = createsession.CreateSession
	}

	if r.Clock == nil {
		r.Clock = clockwork.NewRealClock()
	}

	return nil
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
	SerialNumber string `json:"-"`
}

// GenerateCredentials generates AWS IAM Roles Anywhere credentials for the Application (IAM Roles Anywhere Profile) and Role ARN.
func GenerateCredentials(ctx context.Context, req GenerateCredentialsRequest) (*Credentials, error) {
	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKeyECDSA, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("unexpected private key type %T", privateKey)
	}

	clusterName, err := req.Cache.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsRACA, err := req.Cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.AWSRACA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCert, tlsSigner, err := req.KeyStoreManager.GetTLSCertAndSigner(ctx, awsRACA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certPEMBytes, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     req.Clock,
		PublicKey: privateKey.Public(),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		Subject: pkix.Name{
			CommonName: req.SubjectCommonName,
		},
		// The certificate only needs to be valid for a very short time: the time it takes to exchange it for AWS credentials.
		// Setting it to 1 minutes is enough to account for clock drift and network latency.
		NotAfter: req.Clock.Now().Add(1 * time.Minute),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	x509Cert, _, err := keys.X509Certificate(certPEMBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createSessionReq := createsession.CreateSessionRequest{
		TrustAnchorARN:  req.TrustAnchorARN,
		ProfileARN:      req.ProfileARN,
		RoleARN:         req.RoleARN,
		Certificate:     x509Cert,
		PrivateKey:      privateKeyECDSA,
		DurationSeconds: req.DurationSeconds,
	}

	// Providing a custom role session name to the CreateSessionAPI for a Profile that doesn't accept it, results in an Access Denied error.
	// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-create-session.html#create-session-and-assume-role
	if req.AcceptRoleSessionName {
		createSessionReq.RoleSessionName = req.SubjectCommonName
	}

	createSessionResp, err := req.CreateSession(ctx, createSessionReq)
	if err != nil {
		return nil, trace.BadParameter("failed to create session %v", err)
	}

	return &Credentials{
		Version:         createSessionResp.Version,
		AccessKeyID:     createSessionResp.AccessKeyID,
		SecretAccessKey: createSessionResp.SecretAccessKey,
		SessionToken:    createSessionResp.SessionToken,
		Expiration:      createSessionResp.Expiration,
		SerialNumber:    x509Cert.SerialNumber.String(),
	}, nil
}
