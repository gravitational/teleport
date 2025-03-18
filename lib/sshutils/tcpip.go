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

// ParseTCPIPForwardReq parses an SSH request's payload into a TCPIPForwardReq.
func ParseTCPIPForwardReq(data []byte) (*TCPIPForwardReq, error) {
	var r TCPIPForwardReq
	if err := ssh.Unmarshal(data, &r); err != nil {
		log.Infof("failed to parse TCP IP Forward request: %v", err)
		return nil, trace.Wrap(err)
	}
	return &r, nil
}
