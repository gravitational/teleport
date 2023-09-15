/*
Copyright 2023 Gravitational, Inc.

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

package reversetunnel

import (
	"errors"
	"net"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

var errDirectDialNoProxyRec = errors.New("direct dialing to nodes not found in inventory requires that the session recording mode is set to record at the proxy")

func checkNodeAndRecConfig(params reversetunnelclient.DialParams, recConfig types.SessionRecordingConfig) error {
	if params.IsNotInventoryNode && !services.IsRecordAtProxy(recConfig.GetMode()) {
		return trace.Wrap(errDirectDialNoProxyRec)
	}
	return nil
}

// shouldDialAndForward returns whether a connection should be proxied
// and forwarded or not.
func shouldDialAndForward(params reversetunnelclient.DialParams, recConfig types.SessionRecordingConfig) bool {
	// connection is already being tunneled, do not forward
	if params.FromPeerProxy {
		return false
	}
	// the node is an agentless node, the connection must be forwarded
	if params.TargetServer != nil && params.TargetServer.IsOpenSSHNode() {
		return true
	}
	// proxy session recording mode is being used and an SSH session
	// is being requested, the connection must be forwarded
	if params.ConnType == types.NodeTunnel && services.IsRecordAtProxy(recConfig.GetMode()) {
		return true
	}
	// if the node was directly dialed and not in the inventory, the
	// connection must be forwarded
	if params.IsNotInventoryNode {
		return true
	}

	return false
}

// shouldSendSignedPROXYHeader returns whether a connection should send
// a signed PROXY header at the start of the connection or not.
func shouldSendSignedPROXYHeader(signer multiplexer.PROXYHeaderSigner, useTunnel, isAgentlessNode bool, srcAddr, dstAddr net.Addr) bool {
	// nothing to sign with, can't send a signed header
	if signer == nil {
		return false
	}
	// signed PROXY headers aren't sent over a tunnel
	if useTunnel {
		return false
	}
	// we are connecting to an agentless node which won't understand the
	// PROXY protocol
	if isAgentlessNode {
		return false
	}
	// we have to have both the source and destination to populate the
	// signed PROXY header with if we want to send it
	if srcAddr == nil || dstAddr == nil {
		return false
	}

	return true
}

func isAgentlessNode(params reversetunnelclient.DialParams) bool {
	// If the node is not in the inventory (was directly dialed) tell
	// the forwarding server it isn't an agentless node so config checks
	// pass. params.TargetServer will ensure the node is not treated as
	// a Teleport node in this case.
	return params.IsAgentlessNode && !params.IsNotInventoryNode
}
