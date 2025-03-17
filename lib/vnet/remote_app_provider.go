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

package vnet

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"

	"github.com/gravitational/trace"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// remoteAppProvider implements appProvider when the client application is
// available over gRPC.
type remoteAppProvider struct {
	clt *clientApplicationServiceClient
}

func newRemoteAppProvider(clt *clientApplicationServiceClient) *remoteAppProvider {
	return &remoteAppProvider{
		clt: clt,
	}
}

// ResolveAppInfo implements [appProvider.ResolveAppInfo].
func (p *remoteAppProvider) ResolveAppInfo(ctx context.Context, fqdn string) (*vnetv1.AppInfo, error) {
	appInfo, err := p.clt.ResolveAppInfo(ctx, fqdn)
	// Avoid wrapping errNoTCPHandler, no need to collect a stack trace.
	if errors.Is(err, errNoTCPHandler) {
		return nil, errNoTCPHandler
	}
	return appInfo, trace.Wrap(err)
}

// ReissueAppCert issues a new cert for the target app. Signatures made with the
// returned [tls.Certificate] happen over gRPC as the key never leaves the
// client application process.
func (p *remoteAppProvider) ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) (tls.Certificate, error) {
	cert, err := p.clt.ReissueAppCert(ctx, appInfo, targetPort)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "reissuing certificate for app %s", appInfo.GetAppKey().GetName())
	}
	signer, err := p.newAppCertSigner(cert, appInfo.GetAppKey(), targetPort)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert},
		PrivateKey:  signer,
	}
	return tlsCert, nil
}

func (p *remoteAppProvider) newAppCertSigner(cert []byte, appKey *vnetv1.AppKey, targetPort uint16) (*rpcAppCertSigner, error) {
	x509Cert, err := x509.ParseCertificate(cert)
	if err != nil {
		return nil, trace.Wrap(err, "parsing x509 certificate")
	}
	return &rpcAppCertSigner{
		clt:        p.clt,
		pub:        x509Cert.PublicKey,
		appKey:     appKey,
		targetPort: targetPort,
	}, nil
}

// rpcAppCertSigner implements [crypto.Signer] for app TLS signatures that are
// issued by the client application over gRPC.
type rpcAppCertSigner struct {
	clt        *clientApplicationServiceClient
	pub        crypto.PublicKey
	appKey     *vnetv1.AppKey
	targetPort uint16
}

// Public implements [crypto.Signer.Public] and returns the public key
// associated with the signer.
func (s *rpcAppCertSigner) Public() crypto.PublicKey {
	return s.pub
}

// Sign implements [crypto.Signer.Sign] and issues a signature over digest for
// the associated app.
func (s *rpcAppCertSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	req := &vnetv1.SignForAppRequest{
		AppKey:     s.appKey,
		TargetPort: uint32(s.targetPort),
		Digest:     digest,
	}
	switch opts.HashFunc() {
	case 0:
		req.Hash = vnetv1.Hash_HASH_NONE
	case crypto.SHA256:
		req.Hash = vnetv1.Hash_HASH_SHA256
	default:
		return nil, trace.BadParameter("unsupported signature hash func %v", opts.HashFunc())
	}
	if pssOpts, ok := opts.(*rsa.PSSOptions); ok {
		saltLen := int32(pssOpts.SaltLength)
		req.PssSaltLength = &saltLen
	}
	signature, err := s.clt.SignForApp(context.TODO(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signature, nil
}

// OnNewConnection reports a new TCP connection to the target app.
func (p *remoteAppProvider) OnNewConnection(ctx context.Context, appKey *vnetv1.AppKey) error {
	if err := p.clt.OnNewConnection(ctx, appKey); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// OnInvalidLocalPort reports a failed connection to an invalid local port for
// the target app.
func (p *remoteAppProvider) OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) {
	if err := p.clt.OnInvalidLocalPort(ctx, appInfo, targetPort); err != nil {
		log.ErrorContext(ctx, "Could not notify client application about invalid local port",
			"error", err,
			"app_name", appInfo.GetAppKey().GetName(),
			"target_port", targetPort,
		)
	}
}
