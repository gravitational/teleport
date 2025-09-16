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

package db

import (
	"context"
	"crypto/x509/pkix"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

type CertificateSigner interface {
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)
}

// GenerateDatabaseCertificatesRequest contains the required fields used to generate database certificates
// Those certificates will be used by databases to set up mTLS authentication against Teleport
type GenerateDatabaseCertificatesRequest struct {
	ClusterAPI         CertificateSigner
	Principals         []string
	OutputFormat       identityfile.Format
	OutputCanOverwrite bool
	OutputLocation     string
	IdentityFileWriter identityfile.ConfigWriter
	TTL                time.Duration
	Key                *client.Key
	// Password is used to generate JKS keystore used for cassandra format or Oracle wallet.
	Password string
}

// GenerateDatabaseServerCertificates to be used by databases to set up mTLS authentication
func GenerateDatabaseServerCertificates(ctx context.Context, req GenerateDatabaseCertificatesRequest) ([]string, error) {

	if len(req.Principals) == 0 ||
		(len(req.Principals) == 1 && req.Principals[0] == "" && req.OutputFormat != identityfile.FormatSnowflake) {

		return nil, trace.BadParameter("at least one hostname must be specified")
	}

	// For CockroachDB node certificates, CommonName must be "node":
	//
	// https://www.cockroachlabs.com/docs/v21.1/cockroach-cert#node-key-and-certificates
	if req.OutputFormat == identityfile.FormatCockroach {
		req.Principals = append([]string{"node"}, req.Principals...)
	}

	subject := pkix.Name{CommonName: req.Principals[0]}

	clusterNameType, err := req.ClusterAPI.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName := clusterNameType.GetClusterName()
	if req.OutputFormat == identityfile.FormatMongo {
		// Include Organization attribute in MongoDB certificates as well.
		//
		// When using X.509 member authentication, MongoDB requires O or OU to
		// be non-empty so this will make the certs we generate compatible:
		//
		// https://docs.mongodb.com/manual/core/security-internal-authentication/#x.509
		//
		// The actual O value doesn't matter as long as it matches on all
		// MongoDB cluster members so set it to the Teleport cluster name
		// to avoid hardcoding anything.

		subject.Organization = []string{clusterName}
	}

	if req.Key == nil {
		key, err := client.GenerateRSAKey()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req.Key = key
	}

	csr, err := tlsca.GenerateCertificateRequestPEM(subject, req.Key.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := req.ClusterAPI.GenerateDatabaseCert(ctx,
		&proto.DatabaseCertRequest{
			CSR: csr,
			// Important to include SANs since CommonName has been deprecated
			// since Go 1.15:
			//   https://golang.org/doc/go1.15#commonname
			ServerNames: req.Principals,
			// Include legacy ServerName for compatibility.
			ServerName:    req.Principals[0],
			TTL:           proto.Duration(req.TTL),
			RequesterName: proto.DatabaseCertRequest_TCTL,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// For CockroachDB we provide node.crt, node.key, ca.crt, and ca-client.crt,
	// and the user must use their own CA to issue client.node.crt,
	// client.node.key, and add their own CA's cert to ca-client.crt.
	// The response CA certs are for Teleport DB Client CA, so fetch the
	// Teleport Database CA certs as well.
	var additionalCACerts [][]byte
	if req.OutputFormat == identityfile.FormatCockroach {
		dbServerCA, err := req.ClusterAPI.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.DatabaseCA,
			DomainName: clusterName,
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, keyPair := range dbServerCA.GetTrustedTLSKeyPairs() {
			additionalCACerts = append(additionalCACerts, keyPair.Cert)
		}
	}

	req.Key.TLSCert = resp.Cert
	req.Key.TrustedCerts = []authclient.TrustedCerts{{
		ClusterName:     req.Key.ClusterName,
		TLSCertificates: resp.CACerts,
	}}
	filesWritten, err := identityfile.Write(ctx, identityfile.WriteConfig{
		OutputPath:           req.OutputLocation,
		Key:                  req.Key,
		Format:               req.OutputFormat,
		OverwriteDestination: req.OutputCanOverwrite,
		Writer:               req.IdentityFileWriter,
		Password:             req.Password,
		AdditionalCACerts:    additionalCACerts,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return filesWritten, nil
}
