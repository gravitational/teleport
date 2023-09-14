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

// Functions that are used in both local and remote sites.
package reversetunnel

import (
	"net"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
)

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
	// DELETE in 15.0.0
	if params.IsUnknownNode {
		return true
	}

	return false
}

func shouldSendSignedPROXYHeader(signer multiplexer.PROXYHeaderSigner, useTunnel, isAgentlessNode bool, srcAddr, dstAddr net.Addr) bool {
	return !(signer == nil ||
		useTunnel ||
		isAgentlessNode ||
		srcAddr == nil ||
		dstAddr == nil)
}
