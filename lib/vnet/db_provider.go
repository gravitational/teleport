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

	"github.com/gravitational/trace"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// dbProvider implements methods related to database access. It lives in the
// admin process and communicates with the user process over gRPC.
type dbProvider struct {
	clt *clientApplicationServiceClient
}

func newDBProvider(clt *clientApplicationServiceClient) *dbProvider {
	return &dbProvider{
		clt: clt,
	}
}

// ReissueDBCert issues a new cert for the target database. Signatures made with
// the returned [tls.Certificate] happen over gRPC as the key never leaves the
// client application process.
func (p *dbProvider) ReissueDBCert(ctx context.Context, dbInfo *vnetv1.DatabaseInfo) (tls.Certificate, error) {
	cert, err := p.clt.ReissueDBCert(ctx, dbInfo)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "reissuing certificate for database %s", dbInfo.GetDatabaseKey().GetName())
	}
	signer, err := p.newDBCertSigner(cert, dbInfo)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert},
		PrivateKey:  signer,
	}
	return tlsCert, nil
}

func (p *dbProvider) newDBCertSigner(cert []byte, dbInfo *vnetv1.DatabaseInfo) (*rpcSigner, error) {
	return newRPCCertSigner(cert, func(req *vnetv1.SignRequest) ([]byte, error) {
		return p.clt.SignForDB(context.TODO(), &vnetv1.SignForDBRequest{
			DatabaseKey: dbInfo.GetDatabaseKey(),
			Sign:        req,
		})
	})
}

// OnNewDBConnection reports a new TCP connection to the target database.
func (p *dbProvider) OnNewDBConnection(ctx context.Context, dbKey *vnetv1.DatabaseKey) error {
	if err := p.clt.OnNewDBConnection(ctx, dbKey); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
