// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy"
)

type appMiddleware struct {
	onExpiredCert func(context.Context) (tls.Certificate, error)
	log           *logrus.Entry
}

// OnNewConnection calls m.onExpiredCert to get a fresh cert if the cert has expired and then sets
// it on the local proxy.
// Other middlewares typically also handle MFA here. App access doesn't support per-session MFA yet,
// so detecting expired certs is all this middleware can do.
func (m *appMiddleware) OnNewConnection(ctx context.Context, lp *alpn.LocalProxy, conn net.Conn) error {
	err := lp.CheckCertExpiry()
	if err == nil {
		return nil
	}

	// Return early and don't fire onExpiredCert if certs are invalid but not due to expiry.
	if !errors.As(err, &x509.CertificateInvalidError{}) {
		return trace.Wrap(err)
	}

	m.log.WithError(err).Debug("Gateway certificates have expired")

	cert, err := m.onExpiredCert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	lp.SetCerts([]tls.Certificate{cert})
	return nil
}

// OnStart is a noop. Middlewares used by tsh check cert validity on start. However, in Connect
// there's no flow which would allow the user to create a local proxy without valid certs.
func (m *appMiddleware) OnStart(context.Context, *alpn.LocalProxy) error {
	return nil
}
