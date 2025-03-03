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

package helpers

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

// DiscardServer is a SSH server that discards SSH exec requests and starts
// with the passed in host signer.
type DiscardServer struct {
	sshServer *sshutils.Server
}

func NewDiscardServer(hostSigner ssh.Signer, listener net.Listener) (*DiscardServer, error) {
	ds := &DiscardServer{}

	// create underlying ssh server
	sshServer, err := sshutils.NewServer(
		"integration-discard-server",
		utils.NetAddr{AddrNetwork: "tcp", Addr: listener.Addr().String()},
		ds,
		sshutils.StaticHostSigners(hostSigner),
		sshutils.AuthMethods{
			PublicKey: ds.userKeyAuth,
		},
		sshutils.SetInsecureSkipHostValidation(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := sshServer.SetListener(listener); err != nil {
		return nil, trace.Wrap(err)
	}
	ds.sshServer = sshServer

	return ds, nil
}

func (s *DiscardServer) userKeyAuth(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
	return nil, nil
}

func (s *DiscardServer) Start() error {
	return s.sshServer.Start()
}

func (s *DiscardServer) Stop() {
	s.sshServer.Close()
}

func (s *DiscardServer) HandleNewChan(_ context.Context, ccx *sshutils.ConnectionContext, newChannel ssh.NewChannel) {
	channel, reqs, err := newChannel.Accept()
	if err != nil {
		ccx.ServerConn.Close()
		ccx.NetConn.Close()
		return
	}

	go s.handleChannel(channel, reqs)
}

func (s *DiscardServer) handleChannel(channel ssh.Channel, reqs <-chan *ssh.Request) {
	defer channel.Close()

	for req := range reqs {
		if req.Type == "exec" {
			successPayload := ssh.Marshal(struct{ C uint32 }{C: uint32(0)})
			channel.SendRequest("exit-status", false, successPayload)
			if req.WantReply {
				req.Reply(true, nil)
			}
			return
		}
		if req.WantReply {
			req.Reply(true, nil)
		}
	}
}
