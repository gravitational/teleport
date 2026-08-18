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
	"crypto/tls"
	"time"

	"github.com/gravitational/trace"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// appProvider implements methods related to TCP app access.
type appProvider struct {
	clt *clientApplicationServiceClient
	// signSem bounds the number of concurrent, possibly-abandoned
	// SignForApp calls. See [callWithBoundedAbandon].
	signSem chan struct{}
}

func newAppProvider(clt *clientApplicationServiceClient) *appProvider {
	return &appProvider{
		clt:     clt,
		signSem: make(chan struct{}, maxInFlightTCPConnectionAttempts),
	}
}

// ReissueAppCert issues a new cert for the target app. Signatures made with the
// returned [tls.Certificate] happen over gRPC as the key never leaves the
// client application process.
func (p *appProvider) ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) (tls.Certificate, error) {
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

// signForAppTimeout bounds a single SignForApp call so that a hung signature
// (the app process is reachable at the transport level but does not respond)
// cannot block a TLS handshake indefinitely and leak the connection's
// in-flight forwarder slot. It must stay well below tcpConnectionSetupTimeout;
// SignForApp normally returns immediately, whether it's a local IPC call to
// the client application or an in-process call in embedded mode.
//
// It's a var, not a const, so tests can shrink it.
var signForAppTimeout = 30 * time.Second

func (p *appProvider) newAppCertSigner(cert []byte, appKey *vnetv1.AppKey, targetPort uint16) (*rpcSigner, error) {
	return newRPCCertSigner(cert, func(req *vnetv1.SignRequest) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), signForAppTimeout)
		defer cancel()
		return callWithBoundedAbandon(ctx, p.signSem, func() ([]byte, error) {
			return p.clt.SignForApp(ctx, vnetv1.SignForAppRequest_builder{
				AppKey:     appKey,
				TargetPort: uint32(targetPort),
				Sign:       req,
			}.Build())
		})
	})
}

// OnNewAppConnection reports a new TCP connection to the target app.
func (p *appProvider) OnNewAppConnection(ctx context.Context, appKey *vnetv1.AppKey) error {
	if err := p.clt.OnNewAppConnection(ctx, appKey); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// OnInvalidLocalPort reports a failed connection to an invalid local port for
// the target app.
func (p *appProvider) OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) {
	if err := p.clt.OnInvalidLocalPort(ctx, appInfo, targetPort); err != nil {
		log.ErrorContext(ctx, "Could not notify client application about invalid local port",
			"error", err,
			"app_name", appInfo.GetAppKey().GetName(),
			"target_port", targetPort,
		)
	}
}
