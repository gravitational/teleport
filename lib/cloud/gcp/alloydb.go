// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package gcp

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	alloydbadmin "cloud.google.com/go/alloydb/apiv1beta"
	"cloud.google.com/go/alloydb/apiv1beta/alloydbpb"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
)

type AlloyDBAdminClient interface {
	// GenerateClientCertificate returns a new PEM-encoded client certificate and Root CA suitable for connecting to particular AlloyDB instance.
	GenerateClientCertificate(ctx context.Context, db types.Database, certExpiry time.Time, pkey *keys.PrivateKey) (*tls.Certificate, string, error)
}

// NewAlloyDBAdminClient returns a AlloyDBAdminClient interface.
func NewAlloyDBAdminClient(ctx context.Context) (AlloyDBAdminClient, error) {
	client, err := alloydbadmin.NewAlloyDBAdminClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create AlloyDB Admin API client: %v", err)
	}
	return &gcpAlloyDBAdminClient{client: client}, nil
}

type gcpAlloyDBAdminClient struct {
	client *alloydbadmin.AlloyDBAdminClient
}

func (g *gcpAlloyDBAdminClient) GenerateClientCertificate(ctx context.Context, db types.Database, certExpiry time.Time, pkey *keys.PrivateKey) (*tls.Certificate, string, error) {
	keyPEM, err := keys.MarshalPublicKey(pkey.Public())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	gcp := db.GetGCP()

	req := &alloydbpb.GenerateClientCertificateRequest{
		Parent: fmt.Sprintf(
			"projects/%s/locations/%s/clusters/%s", gcp.ProjectID, gcp.Region, gcp.ClusterID,
		),
		CertDuration:        durationpb.New(time.Until(certExpiry)),
		PublicKey:           string(keyPEM),
		UseMetadataExchange: true,
	}

	resp, err := g.client.GenerateClientCertificate(ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	certPEMBlock := []byte(strings.Join(resp.PemCertificateChain, "\n"))
	clientCert, err := pkey.TLSCertificate(certPEMBlock)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return &clientCert, resp.CaCert, nil
}
