/*
Copyright 2015 Gravitational, Inc.

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

package regular

import (
	"encoding/json"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv"

	"github.com/gravitational/trace"
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
func (t *proxySitesSubsys) Start(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	log.Debugf("proxysites.start(%v)", ctx)
	remoteSites, err := t.srv.tunnelWithRoles(ctx).GetSites()
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
