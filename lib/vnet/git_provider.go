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

// gitProvider implements methods related to git server access. It lives in the
// admin process and communicates with the user process over gRPC.
type gitProvider struct {
	clt *clientApplicationServiceClient
}

func newGitProvider(clt *clientApplicationServiceClient) *gitProvider {
	return &gitProvider{
		clt: clt,
	}
}

// ReissueGitCert issues a new cert for the target git server. Signatures made
// with the returned [tls.Certificate] happen over gRPC as the key never leaves
// the client application process.
func (p *gitProvider) ReissueGitCert(ctx context.Context, gitInfo *vnetv1.GitServerInfo) (tls.Certificate, error) {
	cert, err := p.clt.ReissueGitCert(ctx, gitInfo)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "reissuing certificate for git server %s", gitInfo.GetGitServerKey().GetName())
	}
	signer, err := p.newGitCertSigner(cert, gitInfo)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert},
		PrivateKey:  signer,
	}
	return tlsCert, nil
}

func (p *gitProvider) newGitCertSigner(cert []byte, gitInfo *vnetv1.GitServerInfo) (*rpcSigner, error) {
	return newRPCCertSigner(cert, func(req *vnetv1.SignRequest) ([]byte, error) {
		return p.clt.SignForGit(context.TODO(), &vnetv1.SignForGitRequest{
			GitServerKey: gitInfo.GetGitServerKey(),
			Sign:         req,
		})
	})
}

// OnNewGitConnection reports a new TCP connection to the target git server.
func (p *gitProvider) OnNewGitConnection(ctx context.Context, gitKey *vnetv1.GitServerKey) error {
	if err := p.clt.OnNewGitConnection(ctx, gitKey); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
