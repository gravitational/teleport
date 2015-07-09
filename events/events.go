package events

import (
	"encoding/base64"
	"io"
	"io/ioutil"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

// AuthSuccess indicates a successfull connection and authentication attempt
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

func (*AuthAttempt) Schema() string {
	return "teleport.auth.attempt"
}

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

func (*Exec) Schema() string {
	return "teleport.exec"
}

// Message is a user message sent in a session
type Message struct {
	// User is SSH user
	User string `json:"user"`

	SessionID string `json:"sid"`

	// Message
	Message string `json:"message"`
}

func (*Message) Schema() string {
	return "teleport.message"
}

type SCP struct {
	// User is SSH user
	User string `json:"user"`

	SessionID string `json:"sid"`
}

func (*SCP) Schema() string {
	return "teleport.scp"
}

func NewShell(sid string, conn ssh.ConnMetadata, shell string, log io.Reader, code int, err error) *Shell {
	return &Shell{
		SessionID:  sid,
		Shell:      shell,
		Log:        collectOutput(log),
		Code:       code,
		Error:      errMsg(err),
		User:       conn.User(),
		LocalAddr:  conn.LocalAddr().String(),
		RemoteAddr: conn.RemoteAddr().String(),
	}
}

// Shell is a result of execution of a in interactive shell
type Shell struct {
	SessionID string `json:"sid"`

	// Shell is a shell name
	Shell string `json:"command"`

	// Code is a return code of a shell
	Code int `json:"code"`

	// Error is a error if shell failed to execute
	Error string `json:"error"`

	// Log is a captured session log
	Log string `json:"log"`

	// User is SSH user
	User string `json:"user"`

	// LocalAddr local connecting address
	LocalAddr string `json:"laddr"`

	// RemoteAddr remote connecting address
	RemoteAddr string `json:"raddr"`
}

func (*Shell) Schema() string {
	return "teleport.shell"
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
