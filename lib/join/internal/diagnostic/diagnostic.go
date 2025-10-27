// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package diagnostic

import (
	"log/slog"
	"sync"
	"time"
)

// Diagnostic collects diagnostic information about a join attempt so that it
// can be displayed in log messages and audit events. Because the join process
// is highly concurrent, it provides safe methods for editing and retrieving
// an [Info] struct.
type Diagnostic struct {
	mu   sync.Mutex
	info Info
}

// Info holds diagnostic information for a join attempt.
type Info struct {
	RemoteAddr          string
	ClientVersion       string
	SafeTokenName       string
	Role                string
	RequestedJoinMethod string
	TokenJoinMethod     string
	TokenExpires        time.Time
	NodeName            string
	HostID              string
	SystemRoles         []string
	BotName             string
	BotGeneration       uint64
	BotInstanceID       string
	Error               error
	RawJoinAttrs        any
}

// New returns an empty diagnostic ready to use.
func New() *Diagnostic {
	return &Diagnostic{}
}

// Get returns a copy of the current collected [Info].
func (d *Diagnostic) Get() Info {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.info
}

// Set accepts a function that will be called under a lock to atomically update
// the collected [Info].
func (d *Diagnostic) Set(f func(*Info)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	f(&d.info)
}

// SlogAttrs returns the current collected info as a slice of [slog.Attr]
// suitable for logging. Unset fields will be included as an empty [slog.Attr]
// which are conventionally ignored by log handlers.
func (info *Info) SlogAttrs() []slog.Attr {
	return []slog.Attr{
		stringAttr("error", info.Error.Error()),
		stringAttr("remote_addr", info.RemoteAddr),
		stringAttr("client_version", info.ClientVersion),
		stringAttr("token_name", info.SafeTokenName),
		stringAttr("role", info.Role),
		stringAttr("requested_join_method", info.RequestedJoinMethod),
		stringAttr("token_join_method", info.TokenJoinMethod),
		timeAttr("token_expires", info.TokenExpires),
		stringAttr("node_name", info.NodeName),
		stringAttr("host_id", info.HostID),
		stringSliceAttr("system_roles", info.SystemRoles),
		stringAttr("bot_name", info.BotName),
		uint64Attr("bot_generation", info.BotGeneration),
		stringAttr("bot_instance_id", info.BotInstanceID),
	}
}

func stringAttr(key, value string) slog.Attr {
	if value == "" {
		return slog.Attr{}
	}
	return slog.String(key, value)
}

func timeAttr(key string, t time.Time) slog.Attr {
	if t.IsZero() {
		return slog.Attr{}
	}
	return slog.Time(key, t)
}

func uint64Attr(key string, u uint64) slog.Attr {
	if u == 0 {
		return slog.Attr{}
	}
	return slog.Uint64(key, u)
}

func stringSliceAttr(key string, vals []string) slog.Attr {
	if len(vals) == 0 {
		return slog.Attr{}
	}
	return slog.Any(key, vals)
}
