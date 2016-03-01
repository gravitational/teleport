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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"

	"github.com/docker/docker/pkg/term"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
)

// Session is an interactive collaboration session that represents one
// or many SSH session started by teleport user
type Session struct {
	// ID is a unique session identifier
	ID string `json:"id"`
	// Parties is a list of session parties
	Parties []Party `json:"parties"`
	// Terminal sets terminal properties
	Terminal TerminalParams `json:"terminal"`
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
}

// Party is a participant a user or a script executing some action
// in the context of the session
type Party struct {
	// ID is a unique party id
	ID string `json:"id"`
	// Site is a remote address?
	RemoteAddr string `json:"remote_addr"`
	// User is a teleport user using this session
	User string `json:"user"`
	// ServerAddr is an address of the server
	ServerAddr string `json:"server_addr"`
	// LastActive is a last time this party was active
	LastActive time.Time `json:"last_active"`
}

// String returns debug friendly representation
func (p *Party) String() string {
	return fmt.Sprintf(
		"party(id=%v, remote=%v, user=%v, server=%v, last_active=%v)",
		p.ID, p.RemoteAddr, p.User, p.ServerAddr, p.LastActive,
	)
}

// DefaultLimit is a default limit if it's not set
const DefaultLimit = 100

// TerminalParams holds parameters of the terminal used in session
type TerminalParams struct {
	W int `json:"W"`
	H int `json:"H"`
}

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
	ID             string          `json:"id"`
	Active         *bool           `json:"active"`
	TerminalParams *TerminalParams `json:"terminal_params"`
}

// Check returns nil if request is valid, error otherwize
func (u *UpdateRequest) Check() error {
	if u.ID == "" {
		return trace.Wrap(teleport.BadParameter("id", "session id can not be empty"))
	}
	if u.TerminalParams != nil {
		_, err := NewTerminalParamsFromInt(u.TerminalParams.W, u.TerminalParams.H)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Service is a realtime SSH session service
// that has information about sessions that are in-flight in the
// cluster at the moment
type Service interface {
	// GetSessions returns a list of currently active sessions
	// with all parties involved
	GetSessions() ([]Session, error)
	// GetSession returns a session with it's parties by ID
	GetSession(id string) (*Session, error)
	// CreateSession creates a new active session and it's parameters
	// if term is skipped, terminal size won't be recorded
	CreateSession(sess Session) error
	// UpdateSession updates certain session parameters (last_active, terminal parameters)
	// other parameters will not be updated
	UpdateSession(req UpdateRequest) error
	// UpsertParty upserts active session party
	UpsertParty(id string, p Party, ttl time.Duration) error
}

type server struct {
	bk               backend.JSONCodec
	clock            timetools.TimeProvider
	activeSessionTTL time.Duration
}

// Option is a functional option that can be given to a server
type Option func(s *server) error

// Clock sets up clock for this server, used in tests
func Clock(c timetools.TimeProvider) Option {
	return func(s *server) error {
		s.clock = c
		return nil
	}
}

// ActiveSessionTTL specifies active session ttl
func ActiveSessionTTL(ttl time.Duration) Option {
	return func(s *server) error {
		s.activeSessionTTL = ttl
		return nil
	}
}

// New returns new session server that uses sqlite to manage
// active sessions
func New(bk backend.Backend, opts ...Option) (Service, error) {
	s := &server{
		bk: backend.JSONCodec{Backend: bk},
	}

	for _, o := range opts {
		if err := o(s); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if s.clock == nil {
		s.clock = &timetools.RealTime{}
	}
	if s.activeSessionTTL == 0 {
		s.activeSessionTTL = DefaultActiveSessionTTL
	}
	return s, nil
}

// MaxLimit is max iteration limit
const MaxLimit = 1000

func activeBucket() []string {
	return []string{"sessions", "active"}
}

func partiesBucket(id string) []string {
	return []string{"sessions", "parties", id}
}

func allBucket() []string {
	return []string{"sessions", "all"}
}

// GetSessions returns a list of active sessions
func (s *server) GetSessions() ([]Session, error) {
	keys, err := s.bk.GetKeys(activeBucket())
	if err != nil {
		return nil, err
	}
	out := []Session{}
	for _, sid := range keys {
		se, err := s.GetSession(sid)
		if teleport.IsNotFound(err) {
			continue
		}
		out = append(out, *se)
	}
	return out, nil
}

// GetSession returns the session by it's id
func (s *server) GetSession(id string) (*Session, error) {
	var sess *Session
	err := s.bk.GetJSONVal(allBucket(), id, &sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.bk.GetVal(activeBucket(), id)
	if err != nil {
		if !teleport.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		sess.Active = false
	} else {
		sess.Active = true
	}

	parties, err := s.bk.GetKeys(partiesBucket(id))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, pk := range parties {
		var p *Party
		err := s.bk.GetJSONVal(partiesBucket(id), pk, &p)
		if err != nil {
			if teleport.IsNotFound(err) { // key was expired
				continue
			}
			return nil, err
		}
		sess.Parties = append(sess.Parties, *p)
	}
	return sess, nil
}

const (
	// DefaultActiveSessionTTL is a TTL when session is marked as inactive
	DefaultActiveSessionTTL = 10 * time.Minute
	// DefaultActivePartyTTL is a TTL when party is marked as inactive
	DefaultActivePartyTTL = 10 * time.Second
)

// CreateSession creates a new session if it does not exist, if the session
// exists the function will return AlreadyExists error
// The session will be marked as active for TTL period of time
func (s *server) CreateSession(sess Session) error {
	if sess.ID == "" {
		return trace.Wrap(teleport.BadParameter("id", "session id can not be empty"))
	}
	if sess.Login == "" {
		return trace.Wrap(teleport.BadParameter("login", "session login can not be empty"))
	}
	if sess.Created.IsZero() {
		return trace.Wrap(teleport.BadParameter("created", "can not be empty"))
	}
	if sess.LastActive.IsZero() {
		return trace.Wrap(teleport.BadParameter("last_active", "can not be empty"))
	}
	_, err := NewTerminalParamsFromInt(sess.Terminal.W, sess.Terminal.H)
	if err != nil {
		return trace.Wrap(err)
	}
	sess.Parties = nil
	err = s.bk.CreateJSONVal(allBucket(), sess.ID, sess, backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.bk.UpsertJSONVal(activeBucket(), sess.ID, "active", s.activeSessionTTL)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateSession updates session parameters - can mark it as inactive and update it's terminal parameters
func (s *server) UpdateSession(req UpdateRequest) error {
	lock := "sessions" + req.ID
	s.bk.AcquireLock(lock, time.Second)
	defer s.bk.ReleaseLock(lock)

	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	var sess *Session
	err := s.bk.GetJSONVal(allBucket(), req.ID, &sess)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TerminalParams != nil {
		sess.Terminal = *req.TerminalParams
	}
	sess.Parties = nil
	err = s.bk.UpsertJSONVal(allBucket(), req.ID, sess, backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.Active != nil {
		if !*req.Active {
			err := s.bk.DeleteKey(activeBucket(), req.ID)
			if err != nil {
				if !teleport.IsNotFound(err) {
					return trace.Wrap(err)
				}
			}
		} else {
			err := s.bk.UpsertJSONVal(activeBucket(), req.ID, "active", s.activeSessionTTL)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// UpsertParty updates or inserts active session party and sets TTL for it
// it also updates
func (s *server) UpsertParty(sessionID string, p Party, ttl time.Duration) error {
	err := s.bk.UpsertJSONVal(partiesBucket(sessionID), p.ID, p, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.bk.UpsertJSONVal(activeBucket(),
		sessionID, "active", s.activeSessionTTL)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewTerminalParamsFromUint32 returns new terminal parameters from uint32 width and height
func NewTerminalParamsFromUint32(w uint32, h uint32) (*TerminalParams, error) {
	if w > maxSize || w < minSize {
		return nil, trace.Wrap(teleport.BadParameter("width", "bad width"))
	}
	if h > maxSize || h < minSize {
		return nil, trace.Wrap(teleport.BadParameter("height", "bad height"))
	}
	return &TerminalParams{W: int(w), H: int(h)}, nil
}

// NewTerminalParamsFromInt returns new terminal parameters from int width and height
func NewTerminalParamsFromInt(w int, h int) (*TerminalParams, error) {
	if w > maxSize || w < minSize {
		return nil, trace.Wrap(teleport.BadParameter("width", "bad witdth"))
	}
	if h > maxSize || h < minSize {
		return nil, trace.Wrap(teleport.BadParameter("height", "bad witdth"))
	}
	return &TerminalParams{W: int(w), H: int(h)}, nil
}

const (
	minSize = 1
	maxSize = 4096
)
