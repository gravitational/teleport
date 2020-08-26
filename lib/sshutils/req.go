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
	"encoding/binary"

	"golang.org/x/crypto/ssh"

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

// TerminalModes converts encoded terminal modes into a ssh.TerminalModes map.
// The encoding is described in: https://tools.ietf.org/html/rfc4254#section-8
//
//   All 'encoded terminal modes' (as passed in a pty request) are encoded
//   into a byte stream.  It is intended that the coding be portable
//   across different environments.  The stream consists of opcode-
//   argument pairs wherein the opcode is a byte value.  Opcodes 1 to 159
//   have a single uint32 argument.  Opcodes 160 to 255 are not yet
//   defined, and cause parsing to stop (they should only be used after
//   any other data).  The stream is terminated by opcode TTY_OP_END
//   (0x00).
//
// In practice, this means encoded terminal modes get translated like below:
//
//  0x80 0x00 0x00 0x38 0x40  0x81 0x00 0x00 0x38 0x40  0x35 0x00 0x00 0x00 0x00  0x00
//  |___|__________________|  |___|__________________|  |___|__________________|  |__|
//         0x80: 0x3840              0x81: 0x3840              0x35: 0x00         0x00
//  ssh.TTY_OP_ISPEED: 14400  ssh.TTY_OP_OSPEED: 14400         ssh.ECHO:0
//
func (p *PTYReqParams) TerminalModes() (ssh.TerminalModes, error) {
	terminalModes := ssh.TerminalModes{}

	if len(p.Modes) == 1 && p.Modes[0] == 0 {
		return terminalModes, nil
	}

	chunkSize := 5
	for i := 0; i < len(p.Modes); i = i + chunkSize {
		// the first byte of the chunk is the key
		key := p.Modes[i]

		// a key with value 0 means is the termination of the stream
		if key == 0 {
			break
		}

		// the remaining 4 bytes of the chunk are the value
		if i+chunkSize > len(p.Modes) {
			return nil, trace.BadParameter("invalid terminal modes encoding")
		}
		value := binary.BigEndian.Uint32([]byte(p.Modes[i+1 : i+chunkSize]))

		terminalModes[key] = value
	}

	return terminalModes, nil
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

// ExecReq specifies parameters for a "exec" request.
type ExecReq struct {
	Command string
}

// SubsystemReq specifies the parameters for a "subsystem" request.
type SubsystemReq struct {
	Name string
}

// SessionEnvVar is environment variable for SSH session
const SessionEnvVar = "TELEPORT_SESSION"

const (
	// ExecRequest is a request to run a command.
	ExecRequest = "exec"

	// ShellRequest is a request for a shell.
	ShellRequest = "shell"

	// EnvRequest is a request to set an environment variable.
	EnvRequest = "env"

	// SubsystemRequest is a request to run a subsystem.
	SubsystemRequest = "subsystem"

	// WindowChangeRequest is a request to change window.
	WindowChangeRequest = "window-change"

	// PTYRequest is a request for PTY.
	PTYRequest = "pty-req"

	// AgentForwardRequest is SSH agent request.
	AgentForwardRequest = "auth-agent-req@openssh.com"

	// AuthAgentRequest is a request to a SSH client to open an agent channel.
	AuthAgentRequest = "auth-agent@openssh.com"

	// X11ForwardRequest is a request to initiate X11 forwarding.
	X11ForwardRequest = "x11-req"

	// X11ChannelRequest is the type of an X11 forwarding channel.
	X11ChannelRequest = "x11"
)

const (
	minSize = 1
	maxSize = 4096
)
