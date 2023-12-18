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

package auditd

import (
	"github.com/gravitational/trace"
	"github.com/mdlayher/netlink"
)

// EventType represent auditd message type.
// Values comes from https://github.com/torvalds/linux/blob/08145b087e4481458f6075f3af58021a3cf8a940/include/uapi/linux/audit.h#L54
type EventType int

const (
	AuditGet       EventType = 1000
	AuditUserEnd   EventType = 1106
	AuditUserLogin EventType = 1112
	AuditUserErr   EventType = 1109
)

type ResultType string

const (
	Success ResultType = "success"
	Failed  ResultType = "failed"
)

// UnknownValue is used by auditd when a value is not provided.
const UnknownValue = "?"

var ErrAuditdDisabled = trace.Errorf("auditd is disabled")

// NetlinkConnector implements netlink related functionality.
type NetlinkConnector interface {
	Execute(m netlink.Message) ([]netlink.Message, error)
	Receive() ([]netlink.Message, error)

	Close() error
}

// Message is an audit message. It contains TTY name, users and connection
// information.
type Message struct {
	// SystemUser is a name of Linux user.
	SystemUser string
	// TeleportUser is a name of Teleport user.
	TeleportUser string
	// ConnAddress is an address of incoming connection.
	ConnAddress string
	// TTYName is a name of TTY used by SSH session is allocated, ex: /dev/tty1
	// or 'teleport' if empty.
	TTYName string
}

// SetDefaults set default values to match what OpenSSH does.
func (m *Message) SetDefaults() {
	if m.SystemUser == "" {
		m.SystemUser = UnknownValue
	}

	if m.ConnAddress == "" {
		m.ConnAddress = UnknownValue
	}

	if m.TTYName == "" {
		m.TTYName = "teleport"
	}
}
