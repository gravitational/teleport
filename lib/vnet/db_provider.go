// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package vnet

import (
	"context"
	"crypto/tls"
	"crypto/x509"

	"github.com/gravitational/trace"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// dbProvider implements methods related to database tunnel access.
type dbProvider struct {
	clt *clientApplicationServiceClient
}

func newDBProvider(clt *clientApplicationServiceClient) *dbProvider {
	return &dbProvider{clt: clt}
}

// ReissueDBCert issues a new cert for the target database. Signatures made
// with the returned [tls.Certificate] happen over gRPC as the key never leaves
// the client application process. Also returns the database protocol string
// (e.g. "postgres", "mysql") looked up from the cluster.
func (p *dbProvider) ReissueDBCert(ctx context.Context, dbKey *vnetv1.DBKey, dbName string) (tls.Certificate, string, error) {
	certDER, dbProtocol, err := p.clt.ReissueDBCert(ctx, dbKey, dbName)
	if err != nil {
		return tls.Certificate{}, "", trace.Wrap(err, "reissuing certificate for database %s", dbKey.GetDbServiceName())
	}
	signer, err := p.newDBCertSigner(certDER, dbKey)
	if err != nil {
		return tls.Certificate{}, "", trace.Wrap(err)
	}
	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  signer,
	}, dbProtocol, nil
}

func (p *dbProvider) newDBCertSigner(certDER []byte, dbKey *vnetv1.DBKey) (*rpcSigner, error) {
	x509Cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, trace.Wrap(err, "parsing x509 certificate")
	}
	return &rpcSigner{
		pub: x509Cert.PublicKey,
		sendRequest: func(req *vnetv1.SignRequest) ([]byte, error) {
			return p.clt.SignForDB(context.TODO(), &vnetv1.SignForDBRequest{
				DbKey: dbKey,
				Sign:  req,
			})
		},
	}, nil
}
