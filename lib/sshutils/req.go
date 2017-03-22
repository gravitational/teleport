/*
Copyright 2015 Gravitational, Inc.

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
	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"
)

// EnvReqParams are parameters for env request
type EnvReqParams struct {
	Name  string
	Value string
}

// WinChangeReqParams specifies parameters for window changes
type WinChangeReqParams struct {
	W   uint32
	H   uint32
	Wpx uint32
	Hpx uint32
}

// PTYReqParams specifies parameters for pty change window
type PTYReqParams struct {
	Env   string
	W     uint32
	H     uint32
	Wpx   uint32
	Hpx   uint32
	Modes string
}

// Check validates PTY parameters.
func (p *PTYReqParams) Check() error {
	if p.W > maxSize || p.W < minSize {
		return trace.BadParameter("bad width: %v", p.W)
	}
	if p.H > maxSize || p.H < minSize {
		return trace.BadParameter("bad height: %v", p.H)
	}

	return nil
}

// CheckAndSetDefaults validates PTY parameters and ensures parameters
// are within default values.
func (p *PTYReqParams) CheckAndSetDefaults() error {
	if p.W > maxSize || p.W < minSize {
		p.W = teleport.DefaultTerminalWidth
	}
	if p.H > maxSize || p.H < minSize {
		p.H = teleport.DefaultTerminalHeight
	}

	return nil
}

const (
	// SessionEnvVar is environment variable for SSH session
	SessionEnvVar = "TELEPORT_SESSION"
	// SetEnvReq sets environment requests
	SetEnvReq = "env"
	// WindowChangeReq is a request to change window
	WindowChangeReq = "window-change"
	// PTYReq is a request for PTY
	PTYReq = "pty-req"
	// AgentReq is ssh agent requesst
	AgentReq = "auth-agent-req@openssh.com"
)

const (
	minSize = 1
	maxSize = 4096
)
