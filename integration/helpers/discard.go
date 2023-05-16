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
		[]ssh.Signer{hostSigner},
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
