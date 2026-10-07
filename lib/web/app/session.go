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

package app

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

// session holds a request forwarder and web session for this request.
type session struct {
	// fwd can rewrite and forward requests to the target application.
	fwd *reverseproxy.Forwarder
	// ws represents the services.WebSession this requests belongs to.
	ws types.WebSession
	// transport allows to dial an application server.
	tr *transport
}

// getIdentityFromWebSession parses the TLS certificate from the web session and
// extracts the user identity from its subject.
func getIdentityFromWebSession(ws types.WebSession) (*tlsca.Identity, error) {
	certificate, err := tlsca.ParseCertificatePEM(ws.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(certificate.Subject, certificate.NotAfter)
	return identity, trace.Wrap(err)
}

// newSession creates a new session.
func (h *Handler) newSession(ws types.WebSession) (*session, error) {
	// Extract the identity of the user.
	identity, err := getIdentityFromWebSession(ws)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appServers := newAppServerResolver(h.c.ClusterGetter, identity.RouteToApp, ws.GetName(), h.cache)
	transport, err := newTransport(&transportConfig{
		log:                   h.logger,
		clock:                 h.c.Clock,
		accessPoint:           h.c.AccessPoint,
		cipherSuites:          h.c.CipherSuites,
		identity:              identity,
		appServers:            appServers,
		ws:                    ws,
		clusterName:           h.clusterName,
		integrationAppHandler: h.c.IntegrationAppHandler,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Don't trust any "X-Forward-*" headers the client sends, instead set our own.
	delegate := reverseproxy.NewHeaderRewriter()
	delegate.TrustForwardHeader = false
	hr := common.NewHeaderRewriter(delegate)

	// Create a forwarder that will be used to forward requests.
	fwd, err := reverseproxy.New(
		reverseproxy.WithPassHostHeader(),
		reverseproxy.WithFlushInterval(100*time.Millisecond),
		reverseproxy.WithRoundTripper(transport),
		reverseproxy.WithLogger(h.logger),
		reverseproxy.WithErrorHandler(h.handleForwardError),
		reverseproxy.WithRewriter(hr),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &session{
		fwd: fwd,
		ws:  ws,
		tr:  transport,
	}, nil
}
