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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// NewClientConnWithDeadline establishes new client connection with specified deadline
func NewClientConnWithDeadline(conn net.Conn, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	if config.Timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(config.Timeout))
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	if config.Timeout > 0 {
		conn.SetReadDeadline(time.Time{})
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// ConnectProxyTransport opens a channel over the remote tunnel and connects
// to the requested host.
func ConnectProxyTransport(sconn ssh.Conn, req *DialReq, exclusive bool) (*ChConn, bool, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, false, trace.Wrap(err)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	channel, discard, err := sconn.OpenChannel(constants.ChanTransport, nil)
	if err != nil {
		ssh.DiscardRequests(discard)
		return nil, false, trace.Wrap(err)
	}

	// Send a special SSH out-of-band request called "teleport-transport"
	// the agent on the other side will create a new TCP/IP connection to
	// 'addr' on its network and will start proxying that connection over
	// this SSH channel.
	ok, err := channel.SendRequest(constants.ChanTransportDialReq, true, payload)
	if err != nil {
		return nil, true, trace.Wrap(err)
	}
	if !ok {
		defer channel.Close()

		// Pull the error message from the tunnel client (remote cluster)
		// passed to us via stderr.
		errMessageBytes, _ := ioutil.ReadAll(channel.Stderr())
		errMessage := strings.TrimSpace(string(errMessageBytes))
		if len(errMessage) == 0 {
			errMessage = fmt.Sprintf("failed connecting to %v [%v]", req.Address, req.ServerID)
		}
		return nil, false, trace.Errorf(errMessage)
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
