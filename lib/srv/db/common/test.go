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
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
)

// TestServerConfig combines parameters for a test Postgres/MySQL server.
type TestServerConfig struct {
	// AuthClient will be used to retrieve trusted CA.
	AuthClient auth.ClientI
	// Name is the server name for identification purposes.
	Name string
	// Address is an optional server listen address.
	Address string
	// AuthUser is used in tests simulating IAM token authentication.
	AuthUser string
	// AuthToken is used in tests simulating IAM token authentication.
	AuthToken string
	// CN allows to set specific CommonName in the database server certificate.
	//
	// Used when simulating test Cloud SQL database which should contains
	// <project-id>:<instance-id> in its certificate.
	CN string
}

// MakeTestServerTLSConfig returns TLS config suitable for configuring test
// database Postgres/MySQL servers.
func MakeTestServerTLSConfig(config TestServerConfig) (*tls.Config, error) {
	cn := config.CN
	if cn == "" {
		cn = "localhost"
	}
	privateKey, _, err := testauthority.New().GenerateKeyPair("")
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
			CSR:        csr,
			ServerName: cn,
			TTL:        proto.Duration(time.Hour),
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tls.X509KeyPair(resp.Cert, privateKey)
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
		Certificates: []tls.Certificate{cert},
	}, nil
}

// TestClientConfig combines parameters for a test Postgres/MySQL client.
type TestClientConfig struct {
	// AuthClient will be used to retrieve trusted CA.
	AuthClient auth.ClientI
	// AuthServer will be used to generate database access certificate for a user.
	AuthServer *auth.Server
	// Address is the address to connect to (web proxy).
	Address string
	// Cluster is the Teleport cluster name.
	Cluster string
	// Username is the Teleport user name.
	Username string
	// RouteToDatabase contains database routing information.
	RouteToDatabase tlsca.RouteToDatabase
}

// MakeTestClientTLSConfig returns TLS config suitable for configuring test
// database Postgres/MySQL clients.
func MakeTestClientTLSConfig(config TestClientConfig) (*tls.Config, error) {
	key, err := client.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Generate client certificate for the Teleport user.
	cert, err := config.AuthServer.GenerateDatabaseTestCert(auth.DatabaseTestCertRequest{
		PublicKey:       key.Pub,
		Cluster:         config.Cluster,
		Username:        config.Username,
		RouteToDatabase: config.RouteToDatabase,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tls.X509KeyPair(cert, key.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := config.AuthClient.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
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
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}, nil
}
