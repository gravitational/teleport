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
	"strings"
	"time"

	alloydbadmin "cloud.google.com/go/alloydb/apiv1beta"
	"cloud.google.com/go/alloydb/apiv1beta/alloydbpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	gcputils "github.com/gravitational/teleport/api/utils/gcp"
	"github.com/gravitational/teleport/api/utils/keys"
)

// AlloyDBAdminClient encapsulates alloydb.AlloyDBAdminClient
type AlloyDBAdminClient interface {
	// GenerateClientCertificate returns a new PEM-encoded client certificate and Root CA suitable for connecting to particular AlloyDB instance.
	GenerateClientCertificate(ctx context.Context, info gcputils.AlloyDBFullInstanceName, certExpiry time.Time, pkey *keys.PrivateKey) (*tls.Certificate, string, error)
	// GetEndpointAddress returns endpoint address for given AlloyDB instance and chosen endpoint type.
	GetEndpointAddress(ctx context.Context, info gcputils.AlloyDBFullInstanceName, endpointType string) (string, error)
}

// NewAlloyDBAdminClient returns a AlloyDBAdminClient interface.
func NewAlloyDBAdminClient(ctx context.Context) (AlloyDBAdminClient, error) {
	client, err := alloydbadmin.NewAlloyDBAdminClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &gcpAlloyDBAdminClient{apiClient: client}, nil
}

// alloyDBAdminAPIClient interface lists methods used in *alloydbadmin.AlloyDBAdminClient; used for mocking.
type alloyDBAdminAPIClient interface {
	GenerateClientCertificate(context.Context, *alloydbpb.GenerateClientCertificateRequest, ...gax.CallOption) (*alloydbpb.GenerateClientCertificateResponse, error)
	GetConnectionInfo(context.Context, *alloydbpb.GetConnectionInfoRequest, ...gax.CallOption) (*alloydbpb.ConnectionInfo, error)
}

type gcpAlloyDBAdminClient struct {
	apiClient alloyDBAdminAPIClient
}

// GenerateClientCertificate returns a new PEM-encoded client certificate and Root CA suitable for connecting to particular AlloyDB instance.
//
// See: https://cloud.google.com/go/docs/reference/cloud.google.com/go/alloydb/latest/apiv1beta#cloud_google_com_go_alloydb_apiv1beta_AlloyDBAdminClient_GenerateClientCertificate
func (g *gcpAlloyDBAdminClient) GenerateClientCertificate(ctx context.Context, info gcputils.AlloyDBFullInstanceName, certExpiry time.Time, pkey *keys.PrivateKey) (*tls.Certificate, string, error) {
	keyPEM, err := keys.MarshalPublicKey(pkey.Public())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	req := &alloydbpb.GenerateClientCertificateRequest{
		Parent:              info.ParentClusterName(),
		CertDuration:        durationpb.New(time.Until(certExpiry)),
		PublicKey:           string(keyPEM),
		UseMetadataExchange: true,
	}

	resp, err := g.apiClient.GenerateClientCertificate(ctx, req)
	if err != nil {
		// See: https://cloud.google.com/alloydb/docs/reference/iam-roles-permissions
		// Cloud AlloyDB Client is the least-privileged role with alloydb.clusters.generateClientCertificate permission.
		if strings.Contains(err.Error(), "Permission 'alloydb.clusters.generateClientCertificate' denied") {
			return nil, "", trace.AccessDenied(`Could not generate client certificate:

  %v

Make sure Teleport database agent's IAM user has the 'alloydb.clusters.generateClientCertificate' permission.
Create a custom role with this permission or use the predefined 'Cloud AlloyDB Database User' role.  

Note that IAM changes may take a few minutes to propagate.`, err)
		}
		return nil, "", trace.Wrap(err)
	}

	certPEMBlock := []byte(strings.Join(resp.PemCertificateChain, "\n"))
	clientCert, err := pkey.TLSCertificate(certPEMBlock)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return &clientCert, resp.CaCert, nil
}

// GetEndpointAddress returns endpoint address for given AlloyDB instance and chosen endpoint type. Returns an error if chosen endpoint type is not available.
func (g *gcpAlloyDBAdminClient) GetEndpointAddress(ctx context.Context, info gcputils.AlloyDBFullInstanceName, endpointType string) (string, error) {
	req := &alloydbpb.GetConnectionInfoRequest{Parent: info.InstanceName()}

	resp, err := g.apiClient.GetConnectionInfo(ctx, req)
	if err != nil {
		// See: https://cloud.google.com/alloydb/docs/reference/iam-roles-permissions
		// Cloud AlloyDB Client is the least-privileged role with alloydb.clusters.generateClientCertificate permission.
		if strings.Contains(err.Error(), "Permission 'alloydb.instances.connect' denied") {
			return "", trace.AccessDenied(`Could not generate client certificate:
		
  %v

Make sure Teleport database agent's IAM user has the 'alloydb.instances.connect' permission.
Create a custom role with this permission or use the predefined 'Cloud AlloyDB Database User' role.  

Note that IAM changes may take a few minutes to propagate.`, err)
		}
		return "", trace.Wrap(err)
	}

	var addr string

	switch gcputils.AlloyDBEndpointType(endpointType) {
	case gcputils.AlloyDBEndpointTypePrivate:
		addr = resp.GetIpAddress()
	case gcputils.AlloyDBEndpointTypePublic:
		addr = resp.GetPublicIpAddress()
	case gcputils.AlloyDBEndpointTypePSC:
		addr = resp.GetPscDnsName()
	default:
		return "", trace.BadParameter("unknown endpoint type: %v", endpointType)
	}

	if addr == "" {
		return "", trace.NotFound("endpoint type %v is not available", endpointType)
	}

	return addr, nil
}
