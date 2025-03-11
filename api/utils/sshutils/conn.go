/*
Copyright 2021 Gravitational, Inc.

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

package sshutils

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

// ConnectProxyTransport opens a channel over the remote tunnel and connects
// to the requested host.
//
// Returns the net.Conn wrapper over an SSH channel, whether the provided ssh.Conn
// should be considered invalid due to errors opening or sending a request to the
// channel while setting up the ChConn, and any error that occurs.
func ConnectProxyTransport(sconn ssh.Conn, req *DialReq, exclusive bool) (conn *ChConn, invalid bool, err error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, false, trace.Wrap(err)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	channel, reqC, err := sconn.OpenChannel(constants.ChanTransport, nil)
	if err != nil {
		return nil, true, trace.Wrap(err)
	}

	// DiscardRequests will return when the channel or underlying connection is closed.
	go ssh.DiscardRequests(reqC)

	// Send a special SSH out-of-band request called "teleport-transport"
	// the agent on the other side will create a new TCP/IP connection to
	// 'addr' on its network and will start proxying that connection over
	// this SSH channel.
	ok, err := channel.SendRequest(constants.ChanTransportDialReq, true, payload)
	if err != nil {
		return nil, true, trace.NewAggregate(trace.Wrap(err), channel.Close())
	}
	if !ok {
		defer channel.Close()

		// Pull the error message from the tunnel client (remote cluster)
		// passed to us via stderr.
		errMessageBytes, _ := io.ReadAll(channel.Stderr())
		errMessage := string(bytes.TrimSpace(errMessageBytes))
		if errMessage != "" {
			return nil, false, trace.Errorf("%s", errMessage)
		}

		return nil, false, trace.Errorf("failed connecting to %v [%v]", req.Address, req.ServerID)
	}

	if exclusive {
		return NewExclusiveChConn(sconn, channel), false, nil
	}

	return NewChConn(sconn, channel), false, nil
}

// DialReq is a request for the address to connect to. Supports special
// non-resolvable addresses and search names if connection over a tunnel.
type DialReq struct {
	// Address is the target host to make a connection to.
	Address string `json:"address,omitempty"`

	// ServerID is the hostUUID.clusterName of the node. ServerID is used when
	// dialing through a tunnel to SSH and application nodes.
	ServerID string `json:"server_id,omitempty"`

	// ConnType is the type of connection requested, either node or application.
	ConnType types.TunnelType `json:"conn_type"`

	// ClientSrcAddr is the original observed client address, it is used to propagate
	// correct client IP through indirect connections inside teleport
	ClientSrcAddr string `json:"client_src_addr,omitempty"`

	// ClientDstAddr is the original client's destination address, it is used to propagate
	// correct client point of contact through indirect connections inside teleport
	ClientDstAddr string `json:"client_dst_addr,omitempty"`

	// IsAgentlessNode specifies whether the target is an agentless node.
	IsAgentlessNode bool `json:"is_agentless_node,omitempty"`

	Permit []byte `json:"permit,omitempty"`
}

// CheckAndSetDefaults verifies all the values are valid.
func (d *DialReq) CheckAndSetDefaults() error {
	if d.ConnType == "" {
		d.ConnType = types.NodeTunnel
	}

	if d.Address == "" && d.ServerID == "" {
		return trace.BadParameter("serverID or address required")
	}
	return nil
}
