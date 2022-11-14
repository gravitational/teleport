/*
Copyright 2022 Gravitational, Inc.

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

package gcp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

// SQLAdminClient defines an interface providing access to the GCP Cloud SQL API.
type SQLAdminClient interface {
	// UpdateUser updates an existing user for the project/instance configured in a session.
	UpdateUser(ctx context.Context, db types.Database, dbUser string, user *sqladmin.User) error
	// GetDatabaseInstance returns database instance details for the project/instance
	// configured in a session.
	GetDatabaseInstance(ctx context.Context, db types.Database) (*sqladmin.DatabaseInstance, error)
	// GenerateEphemeralCert returns a new client certificate with RSA key for the
	// project/instance configured in a session.
	GenerateEphemeralCert(ctx context.Context, db types.Database, identity tlsca.Identity) (*tls.Certificate, error)
}

// NewGCPSQLAdminClient returns a GCPSQLAdminClient interface wrapping sqladmin.Service.
func NewSQLAdminClient(ctx context.Context) (SQLAdminClient, error) {
	service, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &gcpSQLAdminClient{service: service}, nil
}

// gcpSQLAdminClient implements the GCPSQLAdminClient interface by wrapping
// sqladmin.Service.
type gcpSQLAdminClient struct {
	service *sqladmin.Service
}

// UpdateUser updates an existing user in a Cloud SQL for the project/instance
// configured in a session.
func (g *gcpSQLAdminClient) UpdateUser(ctx context.Context, db types.Database, dbUser string, user *sqladmin.User) error {
	_, err := g.service.Users.Update(
		db.GetGCP().ProjectID,
		db.GetGCP().InstanceID,
		user).Name(dbUser).Host("%").Context(ctx).Do()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetDatabaseInstance returns database instance details from Cloud SQL for the
// project/instance configured in a session.
func (g *gcpSQLAdminClient) GetDatabaseInstance(ctx context.Context, db types.Database) (*sqladmin.DatabaseInstance, error) {
	gcp := db.GetGCP()
	dbi, err := g.service.Instances.Get(gcp.ProjectID, gcp.InstanceID).Context(ctx).Do()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return dbi, nil
}

// GenerateEphemeralCert returns a new client certificate with RSA key created
// using the GenerateEphemeralCertRequest Cloud SQL API. Client certificates are
// required when enabling SSL in Cloud SQL.
func (g *gcpSQLAdminClient) GenerateEphemeralCert(ctx context.Context, db types.Database, identity tlsca.Identity) (*tls.Certificate, error) {
	// TODO(jimbishopp): cache database certificates to avoid expensive generate
	// operation on each connection.

	// Generate RSA private key, x509 encoded public key, and append to certificate request.
	pkey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pkix, err := x509.MarshalPKIXPublicKey(pkey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Make API call.
	gcp := db.GetGCP()
	req := g.service.Connect.GenerateEphemeralCert(gcp.ProjectID, gcp.InstanceID, &sqladmin.GenerateEphemeralCertRequest{
		PublicKey:     string(pem.EncodeToMemory(&pem.Block{Bytes: pkix, Type: "RSA PUBLIC KEY"})),
		ValidDuration: fmt.Sprintf("%ds", int(time.Until(identity.Expires).Seconds())),
	})
	resp, err := req.Context(ctx).Do()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create TLS certificate from returned ephemeral certificate and private key.
	cert, err := tls.X509KeyPair([]byte(resp.EphemeralCert.Cert), tlsca.MarshalPrivateKeyPEM(pkey))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cert, nil
}
