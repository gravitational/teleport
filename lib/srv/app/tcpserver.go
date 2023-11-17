/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type tcpServer struct {
	newAudit func(sessionID string) (common.Audit, error)
	hostID   string
	log      logrus.FieldLogger
}

// handleConnection handles connection from a TCP application.
func (s *tcpServer) handleConnection(ctx context.Context, clientConn net.Conn, identity *tlsca.Identity, app apitypes.Application) error {
	addr, err := utils.ParseAddr(app.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}
	if addr.AddrNetwork != "tcp" {
		return trace.BadParameter(`unexpected app %q address network, expected "tcp": %+v`, app.GetName(), addr)
	}
	dialer := net.Dialer{
		Timeout: apidefaults.DefaultIOTimeout,
	}
	serverConn, err := dialer.DialContext(ctx, addr.AddrNetwork, addr.String())
	if err != nil {
		return trace.Wrap(err)
	}

	audit, err := s.newAudit(identity.RouteToApp.SessionID)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := audit.OnSessionStart(ctx, s.hostID, identity, app); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := audit.OnSessionEnd(ctx, s.hostID, identity, app); err != nil {
			s.log.WithError(err).Warnf("Failed to emit session end event for app %v.", app.GetName())
		}
	}()
	err = utils.ProxyConn(ctx, clientConn, serverConn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
