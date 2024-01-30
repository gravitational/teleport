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

package sshutils

import (
	"context"
	"net"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// DirectTCPIPReq represents the payload of an SSH "direct-tcpip" or
// "forwarded-tcpip" request.
type DirectTCPIPReq struct {
	// Host is the receiver-side address to forward to.
	Host string
	// Port is the receiver-side port to forward to.
	Port uint32
	// Orig is the sender-side address to forward from.
	Orig string
	// OrigPort is the sender-side port to forward from.
	OrigPort uint32
}

// ParseDirectTCPIPReq parses an SSH request's payload into a DirectTCPIPReq.
func ParseDirectTCPIPReq(data []byte) (*DirectTCPIPReq, error) {
	var r DirectTCPIPReq
	if err := ssh.Unmarshal(data, &r); err != nil {
		log.Infof("failed to parse Direct TCP IP request: %v", err)
		return nil, trace.Wrap(err)
	}
	return &r, nil
}

// TCPIPForwardReq represents the payload of an SSH "tcpip-forward" or
// "cancel-tcpip-forward" request.
type TCPIPForwardReq struct {
	// Addr is the address to listen on.
	Addr string
	// Port is the port to listen on.
	Port uint32
}

func ParseTCPIPForwardReq(data []byte) (*TCPIPForwardReq, error) {
	var r TCPIPForwardReq
	if err := ssh.Unmarshal(data, &r); err != nil {
		log.Infof("failed to parse TCP IP Forward request: %v", err)
		return nil, trace.Wrap(err)
	}
	return &r, nil
}

type CancelTCPIPForwardReq struct {
	Addr string
	Port uint32
}

func StartRemoteListener(ctx context.Context, ccx *ConnectionContext, srcAddr, dstAddr string, listener net.Listener) error {
	srcHost, srcPort, err := SplitHostPort(srcAddr)
	if err != nil {
		return trace.Wrap(err)
	}
	dstHost, dstPort, err := SplitHostPort(dstAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	req := ForwardedTCPIPRequest{
		Addr:     srcHost,
		Port:     srcPort,
		OrigAddr: dstHost,
		OrigPort: dstPort,
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	reqBytes := ssh.Marshal(req)

	go func() {
		for ctx.Err() == nil {
			conn, err := listener.Accept()
			if err != nil {
				log.WithError(err).Warn("failed to accept connection")
				continue
			}

			ch, rch, err := ccx.ServerConn.OpenChannel(teleport.ChanForwardedTCPIP, reqBytes)
			if err != nil {
				log.WithError(err).Warn("failed to open channel")
				continue
			}
			go ssh.DiscardRequests(rch)
			go utils.ProxyConn(ctx, conn, ch)
		}
	}()

	return nil
}
