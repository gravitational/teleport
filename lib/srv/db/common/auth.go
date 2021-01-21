/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/rds/rdsutils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// AuthConfig is the database access authenticator configuration.
type AuthConfig struct {
	// AuthClient is the cluster auth client.
	AuthClient *auth.Client
	// Credentials are the AWS credentials used to generate RDS auth tokens.
	Credentials *credentials.Credentials
	// RDSCACerts contains AWS RDS root certificates.
	RDSCACerts map[string][]byte
	// Clock is the clock implementation.
	Clock clockwork.Clock
	// Log is used for logging.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *AuthConfig) CheckAndSetDefaults() error {
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if c.Credentials == nil {
		return trace.BadParameter("missing Credentials")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "db:auth")
	}
	return nil
}

// Auth provides utilities for creating TLS configurations and
// generating auth tokens when connecting to databases.
type Auth struct {
	cfg AuthConfig
}

// NewAuth returns a new instance of database access authenticator.
func NewAuth(config AuthConfig) (*Auth, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Auth{
		cfg: config,
	}, nil
}

// GetRDSAuthToken returns authorization token that will be used as a password
// when connecting to RDS and Aurora databases.
func (a *Auth) GetRDSAuthToken(sessionCtx *Session) (string, error) {
	a.cfg.Log.Debugf("Generating auth token for %s.", sessionCtx)
	return rdsutils.BuildAuthToken(
		sessionCtx.Server.GetURI(),
		sessionCtx.Server.GetRegion(),
		sessionCtx.DatabaseUser,
		a.cfg.Credentials)
}

// GetTLSConfig builds the client TLS configuration for the session.
//
// For RDS/Aurora, the config must contain RDS root certificate as a trusted
// authority. For onprem we generate a client certificate signed by the host
// CA used to authenticate.
func (a *Auth) GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	addr, err := utils.ParseAddr(sessionCtx.Server.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig := &tls.Config{
		ServerName: addr.Host(),
		RootCAs:    x509.NewCertPool(),
	}
	// Add CA certificate to the trusted pool if it's present, e.g. when
	// connecting to RDS/Aurora which require AWS CA.
	if len(sessionCtx.Server.GetCA()) != 0 {
		if !tlsConfig.RootCAs.AppendCertsFromPEM(sessionCtx.Server.GetCA()) {
			return nil, trace.BadParameter("invalid server CA certificate")
		}
	} else if sessionCtx.Server.IsRDS() {
		if rdsCA, ok := a.cfg.RDSCACerts[sessionCtx.Server.GetRegion()]; ok {
			if !tlsConfig.RootCAs.AppendCertsFromPEM(rdsCA) {
				return nil, trace.BadParameter("invalid RDS CA certificate")
			}
		} else {
			a.cfg.Log.Warnf("No RDS CA certificate for %v.", sessionCtx.Server)
		}
	}
	// RDS/Aurora auth is done via an auth token so don't generate a client
	// certificate and exit here.
	if sessionCtx.Server.IsRDS() {
		return tlsConfig, nil
	}
	// Otherwise, when connecting to an onprem database, generate a client
	// certificate. The database instance should be configured with
	// Teleport's CA obtained with 'tctl auth sign --type=db'.
	cert, cas, err := a.getClientCert(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.Certificates = []tls.Certificate{*cert}
	for _, ca := range cas {
		if !tlsConfig.RootCAs.AppendCertsFromPEM(ca) {
			return nil, trace.BadParameter("failed to append CA certificate to the pool")
		}
	}
	return tlsConfig, nil
}

// getClientCert signs an ephemeral client certificate used by this
// server to authenticate with the database instance.
func (a *Auth) getClientCert(ctx context.Context, sessionCtx *Session) (cert *tls.Certificate, cas [][]byte, err error) {
	privateBytes, _, err := native.GenerateKeyPair("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Postgres requires the database username to be encoded as a common
	// name in the client certificate.
	subject := pkix.Name{CommonName: sessionCtx.DatabaseUser}
	csr, err := tlsca.GenerateCertificateRequestPEM(subject, privateBytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// TODO(r0mant): Cache database certificates to avoid expensive generate
	// operation on each connection.
	a.cfg.Log.Debugf("Generating client certificate for %s.", sessionCtx)
	resp, err := a.cfg.AuthClient.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{
		CSR: csr,
		TTL: proto.Duration(sessionCtx.Identity.Expires.Sub(a.cfg.Clock.Now())),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clientCert, err := tls.X509KeyPair(resp.Cert, privateBytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return &clientCert, resp.CACerts, nil
}
