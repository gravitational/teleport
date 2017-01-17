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

// Package session is used for bookeeping of SSH interactive sessions
// that happen in realtime across the teleport cluster
package session

import (
	"fmt"
	"sort"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/docker/docker/pkg/term"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// ID is a uinique session id that is based on time UUID v1
type ID string

// IsZero returns true if this ID is emtpy
func (s *ID) IsZero() bool {
	return len(*s) == 0
}

// UUID returns byte representation of this ID
func (s *ID) UUID() uuid.UUID {
	return uuid.Parse(string(*s))
}

// String returns string representation of this id
func (s *ID) String() string {
	return string(*s)
}

// Set makes ID cli compatible, lets to set value from string
func (s *ID) Set(v string) error {
	id, err := ParseID(v)
	if err != nil {
		return trace.Wrap(err)
	}
	*s = *id
	return nil
}

// Time returns time portion of this ID
func (s *ID) Time() time.Time {
	tm, ok := s.UUID().Time()
	if !ok {
		return time.Time{}
	}
	sec, nsec := tm.UnixTime()
	return time.Unix(sec, nsec).UTC()
}

// Check checks if it's a valid UUID
func (s *ID) Check() error {
	_, err := ParseID(string(*s))
	return trace.Wrap(err)
}

// ParseID parses ID and checks if it's correct
func ParseID(id string) (*ID, error) {
	val := uuid.Parse(id)
	if val == nil {
		return nil, trace.BadParameter("'%v' is not a valid Time UUID v1", id)
	}
	if ver, ok := val.Version(); !ok || ver != 1 {
		return nil, trace.BadParameter("'%v' is not a be a valid Time UUID v1", id)
	}
	uid := ID(id)
	return &uid, nil
}

// NewID returns new session ID
func NewID() ID {
	return ID(uuid.NewUUID().String())
}

// Session is an interactive collaboration session that represents one
// or many SSH session started by teleport user
type Session struct {
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
	// Active indicates if the session is active
	Active bool `json:"active"`
	// Created records the information about the time when session
	// was created
	Created time.Time `json:"created"`
	// LastActive holds the information about when the session
	// was last active
	LastActive time.Time `json:"last_active"`
	// ServerID
	ServerID string `json:"server_id"`
}

// RemoveParty helper allows to remove a party by it's ID from the
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

// TerminalParams holds parameters of the terminal used in session
type TerminalParams struct {
	W int `json:"w"`
	H int `json:"h"`
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

// Bool returns pointer to a boolean variable
func Bool(val bool) *bool {
	f := val
	return &f
}

// UpdateRequest is a session update request
type UpdateRequest struct {
	ID             ID              `json:"id"`
	Namespace      string          `json:"namespace"`
	Active         *bool           `json:"active"`
	TerminalParams *TerminalParams `json:"terminal_params"`

	// Parties allows to update the list of session parties. nil means
	// "do not update", empty list means "everybody is gone"
	Parties *[]Party `json:"parties"`
}

// Check returns nil if request is valid, error otherwize
func (u *UpdateRequest) Check() error {
	if err := u.ID.Check(); err != nil {
		return trace.Wrap(err)
	}
	if u.Namespace == "" {
		return trace.BadParameter("missing parameter Namespace")
	}
	if u.TerminalParams != nil {
		_, err := NewTerminalParamsFromInt(u.TerminalParams.W, u.TerminalParams.H)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Due to limitations of the current back-end, Teleport won't return more than 1000 sessions
// per time window
const MaxSessionSliceLength = 1000

// Service is a realtime SSH session service
// that has information about sessions that are in-flight in the
// cluster at the moment
type Service interface {
	// GetSessions returns a list of currently active sessions
	// with all parties involved
	GetSessions(namespace string) ([]Session, error)
	// GetSession returns a session with it's parties by ID
	GetSession(namespace string, id ID) (*Session, error)
	// CreateSession creates a new active session and it's parameters
	// if term is skipped, terminal size won't be recorded
	CreateSession(sess Session) error
	// UpdateSession updates certain session parameters (last_active, terminal parameters)
	// other parameters will not be updated
	UpdateSession(req UpdateRequest) error
}

type server struct {
	bk               backend.JSONCodec
	activeSessionTTL time.Duration
}

// New returns new session server that uses sqlite to manage
// active sessions
func New(bk backend.Backend) (Service, error) {
	s := &server{
		bk: backend.JSONCodec{Backend: bk},
	}
	if s.activeSessionTTL == 0 {
		s.activeSessionTTL = defaults.ActiveSessionTTL
	}
	return s, nil
}

func activeBucket(namespace string) []string {
	return []string{"namespaces", namespace, "sessions", "active"}
}

func partiesBucket(namespace string, id ID) []string {
	return []string{"namespaces", namespace, "sessions", "parties", string(id)}
}

// GetSessions returns a list of active sessions. Returns an empty slice
// if no sessions are active
func (s *server) GetSessions(namespace string) ([]Session, error) {
	bucket := activeBucket(namespace)
	out := make(Sessions, 0)

	keys, err := s.bk.GetKeys(bucket)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	for i, sid := range keys {
		if i > MaxSessionSliceLength {
			break
		}
		se, err := s.GetSession(namespace, ID(sid))
		if trace.IsNotFound(err) {
			continue
		}
		out = append(out, *se)
	}
	sort.Stable(out)
	return out, nil
}

// Sessions type is created over []Session to implement sort.Interface to
// be able to sort sessions by creation time
type Sessions []Session

// Swap is part of sort.Interface implementation for []Session
func (slice Sessions) Swap(i, j int) {
	s := slice[i]
	slice[i] = slice[j]
	slice[j] = s
}

// Less is part of sort.Interface implementation for []Session
func (slice Sessions) Less(i, j int) bool {
	return slice[i].Created.Before(slice[j].Created)
}

// Len is part of sort.Interface implementation for []Session
func (slice Sessions) Len() int {
	return len(slice)
}

// GetSession returns the session by it's id. Returns NotFound if a session
// is not found
func (s *server) GetSession(namespace string, id ID) (*Session, error) {
	var sess *Session
	err := s.bk.GetJSONVal(activeBucket(namespace), string(id), &sess)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("session(%v, %v) is not found", namespace, id)
		}
	}
	return sess, nil
}

// CreateSession creates a new session if it does not exist, if the session
// exists the function will return AlreadyExists error
// The session will be marked as active for TTL period of time
func (s *server) CreateSession(sess Session) error {
	if err := sess.ID.Check(); err != nil {
		return trace.Wrap(err)
	}
	if sess.Namespace == "" {
		return trace.BadParameter("session namespace can not be empty")
	}
	if sess.Login == "" {
		return trace.BadParameter("session login can not be empty")
	}
	if sess.Created.IsZero() {
		return trace.BadParameter("created can not be empty")
	}
	if sess.LastActive.IsZero() {
		return trace.BadParameter("last_active can not be empty")
	}
	_, err := NewTerminalParamsFromInt(sess.TerminalParams.W, sess.TerminalParams.H)
	if err != nil {
		return trace.Wrap(err)
	}
	sess.Parties = nil
	err = s.bk.UpsertJSONVal(activeBucket(sess.Namespace), string(sess.ID), sess, s.activeSessionTTL)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateSession updates session parameters - can mark it as inactive and update it's terminal parameters
func (s *server) UpdateSession(req UpdateRequest) error {
	lock := "sessions" + string(req.ID)
	s.bk.AcquireLock(lock, 5*time.Second)
	defer s.bk.ReleaseLock(lock)
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	var sess *Session
	err := s.bk.GetJSONVal(activeBucket(req.Namespace), string(req.ID), &sess)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TerminalParams != nil {
		sess.TerminalParams = *req.TerminalParams
	}
	if req.Active != nil {
		sess.Active = *req.Active
	}
	if req.Parties != nil {
		sess.Parties = *req.Parties
	}
	err = s.bk.UpsertJSONVal(activeBucket(req.Namespace), string(req.ID), sess, s.activeSessionTTL)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

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
		return nil, trace.BadParameter("bad witdth")
	}
	if h > maxSize || h < minSize {
		return nil, trace.BadParameter("bad height")
	}
	return &TerminalParams{W: int(w), H: int(h)}, nil
}

const (
	minSize = 1
	maxSize = 4096
)
