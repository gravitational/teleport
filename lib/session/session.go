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

// Package session is used for bookkeeping of SSH interactive sessions
// that happen in realtime across the teleport cluster
package session

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/moby/term"

	"github.com/gravitational/teleport/api/types"
)

// ID is a unique session ID.
type ID string

// IsZero returns true if this ID is empty.
func (s *ID) IsZero() bool {
	return len(*s) == 0
}

// String returns string representation of this ID.
func (s *ID) String() string {
	return string(*s)
}

// Check will check that the underlying UUID is valid.
func (s *ID) Check() error {
	_, err := ParseID(string(*s))
	return trace.Wrap(err)
}

// ParseID parses ID and checks if it's correct.
func ParseID(id string) (*ID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, trace.BadParameter("%v is not a valid UUID", id)
	}
	// use the parsed UUID to build the ID instead of the string that
	// was passed in. id is user controlled and uuid.Parse accepts
	// several UUID formats that are not supported correctly across
	// Teleport. (uuid.UUID).String always uses the same format that
	// is supported by Teleport everywhere, so use that.
	uid := ID(parsed.String())
	return &uid, nil
}

// NewID returns new session ID. The session ID is based on UUIDv4.
func NewID() ID {
	return ID(uuid.NewString())
}

// Session is a session of any kind (SSH, Kubernetes, Desktop, etc)
type Session struct {
	// Kind describes what kind of session this is e.g. ssh or k8s.
	Kind types.SessionKind `json:"kind"`
	// ID is a unique session identifier
	ID ID `json:"id"`
	// Namespace is a session namespace, separating sessions from each other
	Namespace string `json:"namespace"`
	// Parties is a list of session parties.
	Parties []Party `json:"parties"`
	// TerminalParams sets terminal properties
	TerminalParams TerminalParams `json:"terminal_params"`
	// Login is a login used by all parties joining the session
	Login string `json:"login"`
	// Created records the information about the time when session
	// was created
	Created time.Time `json:"created"`
	// LastActive holds the information about when the session
	// was last active
	LastActive time.Time `json:"last_active"`
	// ServerID of session
	ServerID string `json:"server_id"`
	// ServerHostname of session
	ServerHostname string `json:"server_hostname"`
	// ServerHostPort of session
	ServerHostPort int `json:"server_hostport"`
	// ServerAddr of session
	ServerAddr string `json:"server_addr"`
	// ClusterName is the name of the Teleport cluster that this session belongs to.
	ClusterName string `json:"cluster_name"`
	// KubernetesClusterName is the name of the kube cluster that this session is running in.
	KubernetesClusterName string `json:"kubernetes_cluster_name"`
	// DesktopName is the name of the desktop that this session is running in.
	DesktopName string `json:"desktop_name"`
	// DatabaseName is the name of the database being accessed.
	DatabaseName string `json:"database_name"`
	// AppName is the name of the app being accessed.
	AppName string `json:"app_name"`
	// Owner is the name of the session owner, ie the one who created the session.
	Owner string `json:"owner"`
	// Moderated is true if the session requires moderation (only relevant for Kind = ssh/k8s).
	Moderated bool `json:"moderated"`
	// Command is the command that was executed to start the session.
	Command string `json:"command"`
}

// FileTransferRequestParams contain parameters for requesting a file transfer
type FileTransferRequestParams struct {
	// Download is true if the request is a download, false if it is an upload
	Download bool `json:"direction"`
	// Location is location of file to download, or where to put an upload
	Location string `json:"location"`
	// Filename is the name of the file to be uploaded
	Filename string `json:"filename"`
	// Requester is the authenticated Teleport user who requested the file transfer
	Requester string `json:"requester"`
	// Approvers is a list of teleport users who have approved the file transfer request
	Approvers []Party `json:"approvers"`
}

// FileTransferDecisionParams contains parameters for approving or denying a file transfer request
type FileTransferDecisionParams struct {
	// RequestID is the ID of the request being responded to
	RequestID string `json:"requestId"`
	// Approved is true if the response approves a file transfer request
	Approved bool `json:"approved"`
}

// Participants returns the usernames of the current session participants.
func (s *Session) Participants() []string {
	participants := make([]string, 0, len(s.Parties))
	for _, p := range s.Parties {
		participants = append(participants, p.User)
	}
	return participants
}

// RemoveParty helper allows to remove a party by its ID from the
// session's list. Returns 'false' if pid couldn't be found
func (s *Session) RemoveParty(pid ID) bool {
	for i := range s.Parties {
		if s.Parties[i].ID == pid {
			s.Parties = append(s.Parties[:i], s.Parties[i+1:]...)
			return true
		}
	}
	return false
}

// Party is a participant a user or a script executing some action
// in the context of the session
type Party struct {
	// ID is a unique party id
	ID ID `json:"id"`
	// Site is a remote address?
	RemoteAddr string `json:"remote_addr"`
	// User is a teleport user using this session
	User string `json:"user"`
	// ServerID is an address of the server
	ServerID string `json:"server_id"`
	// LastActive is a last time this party was active
	LastActive time.Time `json:"last_active"`
}

// String returns debug friendly representation
func (p *Party) String() string {
	return fmt.Sprintf(
		"party(id=%v, remote=%v, user=%v, server=%v, last_active=%v)",
		p.ID, p.RemoteAddr, p.User, p.ServerID, p.LastActive,
	)
}

// TerminalParams holds the terminal size in a session.
type TerminalParams struct {
	W int `json:"w"`
	H int `json:"h"`
}

// UnmarshalTerminalParams takes a serialized string that contains the
// terminal parameters and returns a *TerminalParams.
func UnmarshalTerminalParams(s string) (*TerminalParams, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return nil, trace.BadParameter("failed to unmarshal: too many parts")
	}

	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &TerminalParams{
		W: w,
		H: h,
	}, nil
}

// Serialize is a more strict version of String(): it returns a string
// representation of terminal size, this is used in our APIs.
// Format : "W:H"
// Example: "80:25"
func (p *TerminalParams) Serialize() string {
	return fmt.Sprintf("%d:%d", p.W, p.H)
}

// String returns debug friendly representation of terminal
func (p *TerminalParams) String() string {
	return fmt.Sprintf("TerminalParams(w=%v, h=%v)", p.W, p.H)
}

// Winsize returns low-level parameters for changing PTY
func (p *TerminalParams) Winsize() *term.Winsize {
	return &term.Winsize{
		Width:  uint16(p.W),
		Height: uint16(p.H),
	}
}

// MaxSessionSliceLength is the maximum number of sessions per time window
// that the backend will return.
const MaxSessionSliceLength = 1000

// NewTerminalParamsFromUint32 returns new terminal parameters from uint32 width and height
func NewTerminalParamsFromUint32(w uint32, h uint32) (*TerminalParams, error) {
	if w > maxSize || w < minSize {
		return nil, trace.BadParameter("bad width")
	}
	if h > maxSize || h < minSize {
		return nil, trace.BadParameter("bad height")
	}
	return &TerminalParams{W: int(w), H: int(h)}, nil
}

// NewTerminalParamsFromInt returns new terminal parameters from int width and height
func NewTerminalParamsFromInt(w int, h int) (*TerminalParams, error) {
	if w > maxSize || w < minSize {
		return nil, trace.BadParameter("bad width")
	}
	if h > maxSize || h < minSize {
		return nil, trace.BadParameter("bad height")
	}
	return &TerminalParams{W: w, H: h}, nil
}

const (
	minSize = 1
	maxSize = 4096
)
