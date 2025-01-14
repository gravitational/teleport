/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"net"
	"strconv"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/utils"
)

// JoinHostPort is a wrapper for net.JoinHostPort that takes a uint32 port.
func JoinHostPort(host string, port uint32) string {
	return net.JoinHostPort(host, strconv.Itoa(int(port)))
}

// SplitHostPort is a wrapper for net.SplitHostPort that returns a uint32 port.
// Note that unlike net.SplitHostPort, a missing port is valid and will return
// a zero port.
func SplitHostPort(addrString string) (string, uint32, error) {
	addr, err := utils.ParseHostPortAddr(addrString, 0)
	if err != nil {
		return "", 0, trace.Wrap(err)
	}
	return addr.Host(), uint32(addr.Port(0)), nil
}

// SSHConnMetadataWithUser overrides an ssh.ConnMetadata with provided user.
type SSHConnMetadataWithUser struct {
	ssh.ConnMetadata
	user string
}

// NewSSHConnMetadataWithUser overrides an ssh.ConnMetadata with provided user.
func NewSSHConnMetadataWithUser(conn ssh.ConnMetadata, user string) SSHConnMetadataWithUser {
	return SSHConnMetadataWithUser{
		ConnMetadata: conn,
		user:         user,
	}
}

// User returns the user ID for this connection.
func (s SSHConnMetadataWithUser) User() string {
	return s.user
}
