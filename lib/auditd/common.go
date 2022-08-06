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
 * /
 */

package auditd

import (
	"github.com/gravitational/trace"
	"github.com/mdlayher/netlink"
)

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

var ErrAuditdDisabled = trace.Errorf("audutd is disabled")

type NetlinkConnecter interface {
	Execute(m netlink.Message) ([]netlink.Message, error)
	Receive() ([]netlink.Message, error)

	Close() error
}

type Message struct {
	SystemUser   string
	TeleportUser string
	ConnAddress  string
	TTYName      string
}

// SetDefaults set default values to match what OpenSSH does.
func (m *Message) SetDefaults() {
	if m.SystemUser == "" {
		m.SystemUser = "?"
	}

	if m.ConnAddress == "" {
		m.ConnAddress = "?"
	}

	if m.TTYName == "" {
		m.TTYName = "teleport"
	}
}

func eventToOp(event EventType) string {
	switch event {
	case AuditUserEnd:
		return "session_close"
	case AuditUserLogin:
		return "login"
	case AuditUserErr:
		return "invalid_user"
	default:
		return "?"
	}
}
