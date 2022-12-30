/*
 *
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
