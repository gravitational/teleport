/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestServerConfig combines parameters for a test Postgres/MySQL server.
type TestServerConfig struct {
	// AuthClient will be used to retrieve trusted CA.
	AuthClient AuthClientCA
	// Name is the server name for identification purposes.
	Name string
	// AuthUser is used in tests simulating IAM token authentication.
	AuthUser string
	// AuthToken is used in tests simulating IAM token authentication.
	AuthToken string
	// CN allows setting specific CommonName in the database server certificate.
	//
	// Used when simulating test Cloud SQL database which should contains
	// <project-id>:<instance-id> in its certificate.
	CN string
	// ListenTLS creates a TLS listener when true instead of using a net listener.
	// This is used to simulate MySQL connections through the GCP Cloud SQL Proxy.
	ListenTLS bool
	// ClientAuth sets tls.ClientAuth in server's tls.Config. It can be used to force client
	// certificate validation in tests.
	ClientAuth tls.ClientAuthType
	// Users is a list of possible users. If anything provided is outside this list
	// it will return access denied.
	Users []string
	// AllowAnyUser sets the engine to accept any database user.
	AllowAnyUser bool

	Listener net.Listener
}

func (cfg *TestServerConfig) CheckAndSetDefaults() error {
	if cfg.Listener == nil {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Listener = listener
	}

	if cfg.Users == nil {
		cfg.AllowAnyUser = true
	}

	return nil
}

func (cfg *TestServerConfig) CloseOnError(err *error) error {
	if *err != nil {
		return cfg.Close()
	}
	return nil
}

func (cfg *TestServerConfig) Close() error {
	return cfg.Listener.Close()
}

func (cfg *TestServerConfig) Port() (string, error) {
	_, port, err := net.SplitHostPort(cfg.Listener.Addr().String())
	if err != nil {
		return "", trace.Wrap(err)
	}

	return port, nil
}

// AuthClientCA contains the required methods to Generate mTLS certificate to be used
// by the postgres TestServer.
type AuthClientCA interface {
	// GenerateDatabaseCert generates client certificate used by a database
	// service to authenticate with the database instance.
	GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(context.Context, types.CertAuthID, bool) (types.CertAuthority, error)
}

// MakeTestServerTLSConfig returns TLS config suitable for configuring test
// database Postgres/MySQL servers.
func MakeTestServerTLSConfig(config TestServerConfig) (*tls.Config, error) {
	cn := config.CN
	if cn == "" {
		cn = "localhost"
	}
	privateKey, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{
		CommonName: cn,
	}, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := config.AuthClient.GenerateDatabaseCert(context.Background(),
		&proto.DatabaseCertRequest{
			CSR:           csr,
			ServerName:    cn,
			TTL:           proto.Duration(time.Hour),
			RequesterName: proto.DatabaseCertRequest_TCTL,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := privateKey.TLSCertificate(resp.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	for _, ca := range resp.CACerts {
		if ok := pool.AppendCertsFromPEM(ca); !ok {
			return nil, trace.BadParameter("failed to append certificate pem")
		}
	}
	return &tls.Config{
		ClientCAs:    pool,
		ClientAuth:   config.ClientAuth,
		Certificates: []tls.Certificate{cert},
	}, nil
}

// ClientOption represents a database client config option.
type ClientOption func(config *TestClientConfig)

// WithUserAgent set client user agent.
func WithUserAgent(userAgent string) ClientOption {
	return func(config *TestClientConfig) {
		config.UserAgent = userAgent
	}
}

// TestClientConfig combines parameters for a test Postgres/MySQL client.
type TestClientConfig struct {
	// AuthClient will be used to retrieve trusted CA.
	AuthClient authclient.ClientI
	// AuthServer will be used to generate database access certificate for a user.
	AuthServer *auth.Server
	// Address is the address to connect to (web proxy).
	Address string
	// Cluster is the Teleport cluster name.
	Cluster string
	// Username is the Teleport user name.
	Username string
	// PinnedIP is an IP client's certificate should be pinned to.
	PinnedIP string
	// RouteToDatabase contains database routing information.
	RouteToDatabase tlsca.RouteToDatabase
	// UserAgent contains the client user agent.
	UserAgent string
}

// MakeTestClientTLSCert returns TLS certificate suitable for configuring test
// database Postgres/MySQL clients.
func MakeTestClientTLSCert(config TestClientConfig) (*tls.Certificate, error) {
	key, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Generate client certificate for the Teleport user.
	cert, err := config.AuthServer.GenerateDatabaseTestCert(auth.DatabaseTestCertRequest{
		PublicKey:       publicKeyPEM,
		Cluster:         config.Cluster,
		Username:        config.Username,
		RouteToDatabase: config.RouteToDatabase,
		PinnedIP:        config.PinnedIP,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := key.TLSCertificate(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tlsCert, nil
}

// MakeTestClientTLSConfig returns TLS config suitable for configuring test
// database Postgres/MySQL clients.
func MakeTestClientTLSConfig(config TestClientConfig) (*tls.Config, error) {
	tlsCert, err := MakeTestClientTLSCert(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := config.AuthClient.GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: config.Cluster,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool, err := services.CertPool(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tls.Config{
		RootCAs:            pool,
		Certificates:       []tls.Certificate{*tlsCert},
		InsecureSkipVerify: true,
	}, nil
}
