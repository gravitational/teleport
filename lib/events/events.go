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

package events

import (
	"encoding/base64"
	"io"
	"io/ioutil"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	// SessionEvent indicates that session has been initiated
	// or updated by a joining party on the server
	SessionEvent = "teleport.session"
	// SessionEndEvent indicates taht a session has ended
	SessionEndEvent = "teleport.session.end"
	// TerminalResizedEvent fires when the user who initiated the session
	// resizes his terminal
	TerminalResizedEvent = "teleport.resized"
	// ExecEvent is an exec command executed by script or user on
	// the server side
	ExecEvent = "teleport.exec"
	// AuthAttemptEvent is authentication attempt that either
	// succeeded or failed based on event status
	AuthAttemptEvent = "teleport.auth.attempt"
	// SCPEvent means data transfer that occured on the server
	SCPEvent = "teleport.scp"
	// ResizeEvent means that some user resized PTY on the client
	ResizeEvent = "teleport.resize.pty"
)

// AuthAttempt indicates authentication attempt
// that can be either successfull or failed
type AuthAttempt struct {
	// Session is SSH session ID
	SessionID string `json:"sid"`
	// User is SSH user
	User string `json:"user"`
	// Success - true if auth was successfull, false otherwise
	Success bool `json:"success"`
	// Error contains rejection reason if present
	Error string `json:"error"`
	// LocalAddr local connecting address
	LocalAddr string `json:"laddr"`
	// RemoteAddr remote connecting address
	RemoteAddr string `json:"raddr"`
	// Key is a public key used for auth
	Key string `json:"key"`
}

// NewAuthAttempt returns new authentication attempt evetn
func NewAuthAttempt(conn ssh.ConnMetadata, key ssh.PublicKey, success bool, err error) *AuthAttempt {
	return &AuthAttempt{
		SessionID:  string(conn.SessionID()),
		User:       conn.User(),
		Key:        strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
		Success:    success,
		LocalAddr:  conn.LocalAddr().String(),
		RemoteAddr: conn.RemoteAddr().String(),
		Error:      errMsg(err),
	}
}

// Schema returns auth attempt schema
func (*AuthAttempt) Schema() string {
	return AuthAttemptEvent
}

// NewExec returns new Exec event
func NewExec(command string, out io.Reader, code int, err error) *Exec {
	return &Exec{
		Command: command,
		Log:     collectOutput(out),
		Code:    code,
		Error:   errMsg(err),
	}
}

// Exec is a result of execution of a remote command on the target server
type Exec struct {
	// User is SSH user
	User string `json:"user"`
	// SessionID is teleport specific session id
	SessionID string `json:"sid"`
	// Command is a command name with arguments
	Command string `json:"command"`
	// Code is a return code
	Code int `json:"code"`
	// Error is a error if command failed to execute
	Error string `json:"error"`
	// Log is a captured command output
	Log string `json:"out"`
}

// Schema returns event schema
func (*Exec) Schema() string {
	return ExecEvent
}

// Message is a user message sent in a session
type Message struct {
	// User is SSH user
	User string `json:"user"`
	// SessionID is teleport session id
	SessionID string `json:"sid"`
	// Message
	Message string `json:"message"`
}

// Schema return event schema
func (*Message) Schema() string {
	return "teleport.message"
}

// SCP is a file copy event that took place on one of the servers
type SCP struct {
	// User is SSH user
	User string `json:"user"`
	// SessionID is a session id
	SessionID string `json:"sid"`
}

// Schema returns event schema
func (*SCP) Schema() string {
	return SCPEvent
}

// NewShellSession returns a new shell session event
func NewShellSession(sid string, conn ssh.ConnMetadata, shell string, recordID string) *ShellSession {
	return &ShellSession{
		SessionID:  sid,
		Shell:      shell,
		RecordID:   recordID,
		User:       conn.User(),
		LocalAddr:  conn.LocalAddr().String(),
		RemoteAddr: conn.RemoteAddr().String(),
	}
}

type SessionEnded struct {
	SessionID string `json:"sid"`
	ExitCode  int    `json"exit_code"`
	Output    string `json:"output"`
}

func (se *SessionEnded) Schema() string {
	return SessionEndEvent
}

type TerminalResized struct {
	SessionID string `json:"sid"`
	Width     int    `json"W"`
	Height    int    `json:"H"`
}

func (se *TerminalResized) Schema() string {
	return TerminalResizedEvent
}

// ShellSession is a result of execution of an interactive shell
type ShellSession struct {
	// SessionID is teleport session id
	SessionID string `json:"sid"`
	// Shell is a shell name
	Shell string `json:"command"`
	// RecordID holds the id with the session recording
	RecordID string `json:"rid"`
	// User is SSH user
	User string `json:"user"`
	// LocalAddr local connecting address
	LocalAddr string `json:"laddr"`
	// RemoteAddr remote connecting address
	RemoteAddr string `json:"raddr"`
}

// Schema returns event schema
func (*ShellSession) Schema() string {
	return SessionEvent
}

func errMsg(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func collectOutput(r io.Reader) string {
	b, err := ioutil.ReadAll(io.LimitReader(r, 10*1024))
	if err != nil {
		return err.Error()
	}
	return base64.StdEncoding.EncodeToString(b)
}
