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

package regular

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv"
)

// proxySubsys is an SSH subsystem for easy proxyneling through proxy server
// This subsystem creates a new TCP connection and connects ssh channel
// with this connection
type proxySitesSubsys struct {
	srv *Server
}

func parseProxySitesSubsys(name string, srv *Server) (*proxySitesSubsys, error) {
	return &proxySitesSubsys{
		srv: srv,
	}, nil
}

func (t *proxySitesSubsys) String() string {
	return "proxySites()"
}

func (t *proxySitesSubsys) Wait() error {
	return nil
}

// Start serves a request for "proxysites" custom SSH subsystem. It builds an array of
// service.Site structures, and writes it serialized as JSON back to the SSH client
func (t *proxySitesSubsys) Start(ctx context.Context, sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, serverContext *srv.ServerContext) error {
	t.srv.logger.DebugContext(ctx, "starting proxysites subsystem", "server_context", serverContext)
	checker, err := t.srv.tunnelWithAccessChecker(serverContext)
	if err != nil {
		return trace.Wrap(err)
	}

	remoteSites, err := checker.GetSites()
	if err != nil {
		return trace.Wrap(err)
	}

	// build an arary of services.Site structures:
	retval := make([]types.Site, 0, len(remoteSites))
	for _, s := range remoteSites {
		retval = append(retval, types.Site{
			Name:          s.GetName(),
			Status:        s.GetStatus(),
			LastConnected: s.GetLastConnected(),
		})
	}
	// serialize them into JSON and write back:
	data, err := json.Marshal(retval)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := ch.Write(data); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
