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

// EnvReqParams are parameters for env request
type EnvReqParams struct {
	Name  string
	Value string
}

// WinChangeReqParams specifies parameters for window changes
type WinChangeReqParams struct {
	W     uint32
	H     uint32
	Wpx   uint32
	Hpx   uint32
	Modes string
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
