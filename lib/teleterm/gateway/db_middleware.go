// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gateway

import (
	"context"
	"crypto/x509"
	"errors"
	"net"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
)

type dbMiddleware struct {
	onExpiredCert func(context.Context) error
	log           *logrus.Entry
	dbRoute       tlsca.RouteToDatabase
}

// OnNewConnection calls m.onExpiredCert if the cert used by the local proxy has expired.
// This is a very basic reimplementation of client.DBCertChecker.OnNewConnection. DBCertChecker
// supports per-session MFA while for now Connect needs to just check for expired certs.
//
// In the future, DBCertChecker is going to be extended so that it's used by both tsh and Connect
// and this middleware will be removed.
func (m *dbMiddleware) OnNewConnection(ctx context.Context, lp *alpn.LocalProxy, conn net.Conn) error {
	err := lp.CheckDBCerts(m.dbRoute)
	if err == nil {
		return nil
	}

	// Return early and don't fire onExpiredCert if certs are invalid but not due to expiry.
	if !errors.As(err, &x509.CertificateInvalidError{}) {
		return trace.Wrap(err)
	}

	m.log.WithError(err).Debug("Gateway certificates have expired")

	return trace.Wrap(m.onExpiredCert(ctx))
}

// OnStart is a noop. client.DBCertChecker.OnStart checks cert validity too. However in Connect
// there's no flow which would allow the user to create a local proxy without valid
// certs.
func (m *dbMiddleware) OnStart(context.Context, *alpn.LocalProxy) error {
	return nil
}
