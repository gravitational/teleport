/*
Copyright 2015-2018 Gravitational, Inc.

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
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/moby/term"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"
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
	_, err := uuid.Parse(id)
	if err != nil {
		return nil, trace.BadParameter("%v not a valid UUID", id)
	}
	uid := ID(id)
	return &uid, nil
}

// NewID returns new session ID. The session ID is based on UUIDv4.
func NewID() ID {
	return ID(uuid.New().String())
}

// Session is an interactive collaboration session that represents one
// or many sessions started by the teleport user.
type Session struct {
	// Kind describes what kind of session this is e.g. ssh or kubernetes.
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
	// ServerAddr of session
	ServerAddr string `json:"server_addr"`
	// ClusterName is the name of cluster that this session belongs to.
	ClusterName string `json:"cluster_name"`
	// KubernetesClusterName is the name of the kube cluster that this session is running in.
	KubernetesClusterName string `json:"kubernetes_cluster_name"`
}

// Participants returns the usernames of the current session participants.
func (s *Session) Participants() []string {
	participants := make([]string, 0, len(s.Parties))
	for _, p := range s.Parties {
		participants = append(participants, p.User)
	}
	return participants
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

// UpdateRequest is a session update request
type UpdateRequest struct {
	ID             ID              `json:"id"`
	Namespace      string          `json:"namespace"`
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

// MaxSessionSliceLength is the maximum number of sessions per time window
// that the backend will return.
const MaxSessionSliceLength = 1000

// Service is a realtime SSH session service that has information about
// sessions that are in-flight in the cluster at the moment.
type Service interface {
	// GetSessions returns a list of currently active sessions matching
	// the given condition.
	GetSessions(ctx context.Context, namespace string) ([]Session, error)

	// GetSession returns a session with its parties by ID.
	GetSession(ctx context.Context, namespace string, id ID) (*Session, error)

	// CreateSession creates a new active session and it's parameters if term is
	// skipped, terminal size won't be recorded.
	CreateSession(ctx context.Context, sess Session) error

	// UpdateSession updates certain session parameters (last_active, terminal
	// parameters) other parameters will not be updated.
	UpdateSession(ctx context.Context, req UpdateRequest) error

	// DeleteSession removes an active session from the backend.
	DeleteSession(ctx context.Context, namespace string, id ID) error
}

type server struct {
	bk               backend.Backend
	activeSessionTTL time.Duration
	clock            clockwork.Clock
}

// New returns new session server that uses sqlite to manage
// active sessions
func New(bk backend.Backend) (Service, error) {
	s := &server{
		bk:    bk,
		clock: clockwork.NewRealClock(),
	}
	if s.activeSessionTTL == 0 {
		s.activeSessionTTL = defaults.ActiveSessionTTL
	}
	return s, nil
}

func activePrefix(namespace string) []byte {
	return backend.Key("namespaces", namespace, "sessions", "active")
}

func activeKey(namespace string, key string) []byte {
	return backend.Key("namespaces", namespace, "sessions", "active", key)
}

// GetSessions returns a list of active sessions.
// Returns an empty slice if no sessions are active
func (s *server) GetSessions(ctx context.Context, namespace string) ([]Session, error) {
	prefix := activePrefix(namespace)
	result, err := s.bk.GetRange(ctx, prefix, backend.RangeEnd(prefix), MaxSessionSliceLength)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions := make(Sessions, len(result.Items))
	for i, item := range result.Items {
		if err := json.Unmarshal(item.Value, &sessions[i]); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	sort.Stable(sessions)
	return sessions, nil
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

// GetSession returns the session by its id. Returns NotFound if a session
// is not found
func (s *server) GetSession(ctx context.Context, namespace string, id ID) (*Session, error) {
	item, err := s.bk.Get(ctx, activeKey(namespace, string(id)))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("session(%v, %v) is not found", namespace, id)
		}
		return nil, trace.Wrap(err)
	}
	var sess Session
	if err := json.Unmarshal(item.Value, &sess); err != nil {
		return nil, trace.Wrap(err)
	}
	return &sess, nil
}

// CreateSession creates a new session if it does not exist, if the session
// exists the function will return AlreadyExists error
// The session will be marked as active for TTL period of time
func (s *server) CreateSession(ctx context.Context, sess Session) error {
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
	data, err := json.Marshal(sess)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     activeKey(sess.Namespace, string(sess.ID)),
		Value:   data,
		Expires: s.clock.Now().UTC().Add(s.activeSessionTTL),
	}
	_, err = s.bk.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	sessionUpdateAttempts    = 10
	sessionUpdateRetryPeriod = 20 * time.Millisecond
)

// UpdateSession updates session parameters - can mark it as inactive and update it's terminal parameters
func (s *server) UpdateSession(ctx context.Context, req UpdateRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}

	key := activeKey(req.Namespace, string(req.ID))

	// Try several times, then give up
	for i := 0; i < sessionUpdateAttempts; i++ {
		item, err := s.bk.Get(ctx, key)
		if err != nil {
			return trace.Wrap(err)
		}

		var session Session
		if err := json.Unmarshal(item.Value, &session); err != nil {
			return trace.Wrap(err)
		}

		if req.TerminalParams != nil {
			session.TerminalParams = *req.TerminalParams
		}
		if req.Parties != nil {
			session.Parties = *req.Parties
		}
		newValue, err := json.Marshal(session)
		if err != nil {
			return trace.Wrap(err)
		}
		newItem := backend.Item{
			Key:     key,
			Value:   newValue,
			Expires: s.clock.Now().UTC().Add(s.activeSessionTTL),
		}

		_, err = s.bk.CompareAndSwap(ctx, *item, newItem)
		if err != nil {
			if trace.IsCompareFailed(err) || trace.IsConnectionProblem(err) {
				s.clock.Sleep(sessionUpdateRetryPeriod)
				continue
			}
			return trace.Wrap(err)
		}
		return nil
	}
	return trace.ConnectionProblem(nil, "failed concurrently update the session")
}

// DeleteSession removes an active session from the backend.
func (s *server) DeleteSession(ctx context.Context, namespace string, id ID) error {
	if !types.IsValidNamespace(namespace) {
		return trace.BadParameter("invalid namespace %q", namespace)
	}
	err := id.Check()
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.bk.Delete(ctx, activeKey(namespace, string(id)))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// discardSessionServer discards all information about sessions given to it.
type discardSessionServer struct {
}

// NewDiscardSessionServer returns a new discarding session server. It's used
// with the recording proxy so that nodes don't register active sessions to
// the backend.
func NewDiscardSessionServer() Service {
	return &discardSessionServer{}
}

// GetSessions returns an empty list of sessions.
func (d *discardSessionServer) GetSessions(ctx context.Context, namespace string) ([]Session, error) {
	return []Session{}, nil
}

// GetSession always returns a zero session.
func (d *discardSessionServer) GetSession(ctx context.Context, namespace string, id ID) (*Session, error) {
	return &Session{}, nil
}

// CreateSession always returns nil, does nothing.
func (d *discardSessionServer) CreateSession(ctx context.Context, sess Session) error {
	return nil
}

// UpdateSession always returns nil, does nothing.
func (d *discardSessionServer) UpdateSession(ctx context.Context, req UpdateRequest) error {
	return nil
}

// DeleteSession removes an active session from the backend.
func (d *discardSessionServer) DeleteSession(ctx context.Context, namespace string, id ID) error {
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
	return &TerminalParams{W: w, H: h}, nil
}

const (
	minSize = 1
	maxSize = 4096
)
