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

package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
)

type dbMiddleware struct {
	onExpiredCert func(context.Context) (tls.Certificate, error)
	logger        *slog.Logger
	dbRoute       tlsca.RouteToDatabase
}

// OnNewConnection calls m.onExpiredCert if the cert used by the local proxy has expired.
// This is a very basic reimplementation of client.DBCertChecker.OnNewConnection. DBCertChecker
// supports per-session MFA while for now Connect needs to just check for expired certs.
//
// In the future, DBCertChecker is going to be extended so that it's used by both tsh and Connect
// and this middleware will be removed.
func (m *dbMiddleware) OnNewConnection(ctx context.Context, lp *alpn.LocalProxy) error {
	err := lp.CheckDBCert(ctx, m.dbRoute)
	if err == nil {
		return nil
	}

	// Return early and don't fire onExpiredCert if certs are invalid but not due to expiry.
	if !errors.As(err, &x509.CertificateInvalidError{}) {
		return trace.Wrap(err)
	}

	m.logger.DebugContext(ctx, "Gateway certificates have expired", "error", err)

	cert, err := m.onExpiredCert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	lp.SetCert(cert)
	return nil
}

// OnStart is a noop. client.DBCertChecker.OnStart checks cert validity. However in Connect there's
// no flow which would allow the user to create a local proxy without valid certs.
func (m *dbMiddleware) OnStart(context.Context, *alpn.LocalProxy) error {
	return nil
}
