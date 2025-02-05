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

package srv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/moby/term"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/utils"
)

const sessionRecorderID = "session-recorder"

const (
	PresenceVerifyInterval = time.Second * 15
	PresenceMaxDifference  = time.Minute
)

const (
	// sessionRecordingWarningMessage is sent when the session recording is
	// going to be disabled.
	sessionRecordingWarningMessage = "Warning: node error. This might cause some functionalities not to work correctly."
	// sessionRecordingErrorMessage is sent when session recording has some
	// error and the session is terminated.
	sessionRecordingErrorMessage = "Session terminating due to node error."
)

var serverSessions = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: teleport.MetricServerInteractiveSessions,
		Help: "Number of active sessions to this host",
	},
)

func MsgParticipantCtrls(w io.Writer, m types.SessionParticipantMode) error {
	var modeCtrl bytes.Buffer
	modeCtrl.WriteString(fmt.Sprintf("\r\nTeleport > Joining session with participant mode: %s\r\n", string(m)))
	modeCtrl.WriteString("Teleport > Controls\r\n")
	modeCtrl.WriteString("Teleport >   - CTRL-C: Leave the session\r\n")
	if m == types.SessionModeratorMode {
		modeCtrl.WriteString("Teleport >   - t: Forcefully terminate the session\r\n")
	}
	_, err := w.Write(modeCtrl.Bytes())
	if err != nil {
		return fmt.Errorf("could not write bytes: %w", err)
	}
	return nil
}

// SessionRegistry holds a map of all active sessions on a given
// SSH server
type SessionRegistry struct {
	SessionRegistryConfig

	// logger holds the structured logger
	logger *slog.Logger

	// sessions holds a map between session ID and the session object. Used to
	// find active sessions as well as close all sessions when the registry
	// is closing.
	sessions    map[rsession.ID]*session
	sessionsMux sync.Mutex

	// users is used for automatic user creation when new sessions are
	// started
	users HostUsers

	// sudoers is used to create sudoers files at session start
	sudoers        HostSudoers
	sessionsByUser *userSessions
}

type userSessions struct {
	sessionsByUser map[string]int
	m              sync.Mutex
}

func (us *userSessions) add(user string) {
	us.m.Lock()
	defer us.m.Unlock()
	count := us.sessionsByUser[user]
	us.sessionsByUser[user] = count + 1
}

func (us *userSessions) del(user string) int {
	us.m.Lock()
	defer us.m.Unlock()
	count := us.sessionsByUser[user]
	count -= 1
	us.sessionsByUser[user] = count
	return count
}

type SessionRegistryConfig struct {
	// clock is the registry's internal clock. used in testing.
	clock clockwork.Clock

	// srv refers to the upon which this session registry is created.
	Srv Server

	// sessiontrackerService is used to share session activity to
	// other teleport components through the auth server.
	SessionTrackerService services.SessionTrackerService
}

func (sc *SessionRegistryConfig) CheckAndSetDefaults() error {
	if sc.SessionTrackerService == nil {
		return trace.BadParameter("session tracker service is required")
	}

	if sc.Srv == nil {
		return trace.BadParameter("server is required")
	}

	if sc.clock == nil {
		sc.clock = sc.Srv.GetClock()
	}

	return nil
}

func NewSessionRegistry(cfg SessionRegistryConfig) (*SessionRegistry, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	err := metrics.RegisterPrometheusCollectors(serverSessions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SessionRegistry{
		SessionRegistryConfig: cfg,
		logger:                slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentSession, cfg.Srv.Component())),
		sessions:              make(map[rsession.ID]*session),
		users:                 cfg.Srv.GetHostUsers(),
		sudoers:               cfg.Srv.GetHostSudoers(),
		sessionsByUser: &userSessions{
			sessionsByUser: make(map[string]int),
		},
	}, nil
}

func (s *SessionRegistry) addSession(sess *session) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()
	s.sessions[sess.id] = sess
}

func (s *SessionRegistry) removeSession(sess *session) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()
	delete(s.sessions, sess.id)
}

func (s *SessionRegistry) findSessionLocked(id rsession.ID) (*session, bool) {
	sess, found := s.sessions[id]
	return sess, found
}

func (s *SessionRegistry) findSession(id rsession.ID) (*session, bool) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()
	return s.findSessionLocked(id)
}

func (s *SessionRegistry) Close() {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()

	// End all sessions and allow session cleanup
	// goroutines to complete.
	for _, se := range s.sessions {
		se.Stop()
	}

	s.logger.DebugContext(s.Srv.Context(), "Closing Session Registry.")
}

type sudoersCloser struct {
	username     string
	userSessions *userSessions
	cleanup      func(name string) error
}

func (sc *sudoersCloser) Close() error {
	count := sc.userSessions.del(sc.username)
	if count != 0 {
		return nil
	}
	if err := sc.cleanup(sc.username); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// WriteSudoersFile tries to write the needed sudoers entry to the sudoers
// file, if any. If the returned closer is not nil, it must be called at the
// end of the session to cleanup the sudoers file.
func (s *SessionRegistry) WriteSudoersFile(identityContext IdentityContext) (io.Closer, error) {
	if identityContext.Login == teleport.SSHSessionJoinPrincipal {
		return nil, nil
	}

	// Pulling sudoers directly from the Srv so WriteSudoersFile always
	// respects the invariant that we shouldn't write sudoers on proxy servers.
	// This might invalidate the cached sudoers field on SessionRegistry, so
	// we may be able to remove that in a future PR
	sudoWriter := s.Srv.GetHostSudoers()
	if sudoWriter == nil {
		return nil, nil
	}

	sudoers, err := identityContext.AccessChecker.HostSudoers(s.Srv.GetInfo())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(sudoers) == 0 {
		// not an error, sudoers may not be configured.
		return nil, nil
	}
	if err := sudoWriter.WriteSudoers(identityContext.Login, sudoers); err != nil {
		return nil, trace.Wrap(err)
	}

	s.sessionsByUser.add(identityContext.Login)

	return &sudoersCloser{
		username:     identityContext.Login,
		userSessions: s.sessionsByUser,
		cleanup:      sudoWriter.RemoveSudoers,
	}, nil
}

// UpsertHostUser attempts to create or update a local user on the host if needed.
// If the returned closer is not nil, it must be called at the end of the session to
// clean up the local user.
func (s *SessionRegistry) UpsertHostUser(identityContext IdentityContext) (bool, io.Closer, error) {
	ctx := s.Srv.Context()
	log := s.logger.With("host_username", identityContext.Login)

	if identityContext.Login == teleport.SSHSessionJoinPrincipal {
		return false, nil, nil
	}

	if !s.Srv.GetCreateHostUser() || s.users == nil {
		log.DebugContext(ctx, "Not creating host user: node has disabled host user creation.")
		return false, nil, nil // not an error to not be able to create host users
	}

	log.DebugContext(ctx, "Attempting to upsert host user")
	ui, accessErr := identityContext.AccessChecker.HostUsers(s.Srv.GetInfo())
	if trace.IsAccessDenied(accessErr) {
		existsErr := s.users.UserExists(identityContext.Login)
		if existsErr != nil {
			if trace.IsNotFound(existsErr) {
				return false, nil, trace.WrapWithMessage(accessErr, "insufficient permissions for host user creation")
			}

			return false, nil, trace.Wrap(existsErr)
		}
	}

	if accessErr != nil {
		return false, nil, trace.Wrap(accessErr)
	}

	userCloser, err := s.users.UpsertUser(identityContext.Login, *ui)
	if err != nil {
		log.DebugContext(ctx, "Error creating user", "error", err)

		if errors.Is(err, unmanagedUserErr) {
			log.WarnContext(ctx, "User is not managed by teleport. Either manually delete the user from this machine or update the host_groups defined in their role to include 'teleport-keep'. https://goteleport.com/docs/enroll-resources/server-access/guides/host-user-creation/#migrating-unmanaged-users")
			return false, nil, nil
		}

		if !trace.IsAlreadyExists(err) {
			return false, nil, trace.Wrap(err)
		}
		log.DebugContext(ctx, "Host user already exists")
	}

	return true, userCloser, nil
}

// OpenSession either joins an existing active session or starts a new session.
func (s *SessionRegistry) OpenSession(ctx context.Context, ch ssh.Channel, scx *ServerContext) error {
	session := scx.getSession()
	if session != nil && !session.isStopped() {
		scx.Infof("Joining existing session %v.", session.id)
		mode := types.SessionParticipantMode(scx.env[teleport.EnvSSHJoinMode])
		if mode == "" {
			mode = types.SessionPeerMode
		}

		switch mode {
		case types.SessionModeratorMode, types.SessionObserverMode, types.SessionPeerMode:
		default:
			return trace.BadParameter("Unrecognized session participant mode: %v", mode)
		}

		// Update the in-memory data structure that a party member has joined.
		if err := session.join(ch, scx, mode); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	if scx.JoinOnly {
		return trace.AccessDenied("join-only mode was used to create this connection but attempted to create a new session.")
	}

	sid := scx.SessionID()
	if sid.IsZero() {
		return trace.BadParameter("session ID is not set")
	}

	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition
	sess, p, err := newSession(ctx, sid, s, scx, ch, sessionTypeInteractive)
	if err != nil {
		return trace.Wrap(err)
	}
	scx.setSession(sess, ch)
	s.addSession(sess)
	scx.Infof("Creating (interactive) session %v.", sid)

	// Start an interactive session (TTY attached). Close the session if an error
	// occurs, otherwise it will be closed by the callee.
	if err := sess.startInteractive(ctx, scx, p); err != nil {
		sess.Close()
		return trace.Wrap(err)
	}
	return nil
}

// OpenExecSession opens a non-interactive exec session.
func (s *SessionRegistry) OpenExecSession(ctx context.Context, channel ssh.Channel, scx *ServerContext) error {
	sessionID := scx.SessionID()

	if sessionID.IsZero() {
		sessionID = rsession.NewID()
		scx.Tracef("Session not found, creating a new session %s", sessionID)
	} else {
		// Use passed session ID. Assist uses this "feature" to record
		// the execution output.
		scx.Tracef("Session found, reusing it %s", sessionID)
	}

	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition.
	sess, _, err := newSession(ctx, sessionID, s, scx, channel, sessionTypeNonInteractive)
	if err != nil {
		return trace.Wrap(err)
	}
	scx.Infof("Creating (exec) session %v.", sessionID)

	approved, err := s.isApprovedFileTransfer(scx)
	if err != nil {
		return trace.Wrap(err)
	}

	sess.mu.Lock()
	canStart, _, err := sess.checkIfStartUnderLock()
	sess.mu.Unlock()
	if err != nil {
		return trace.Wrap(err)
	}

	// canStart will be true for non-moderated sessions. If canStart is false, check to
	// see if the request has been approved through a moderated session next.
	if !canStart && !approved {
		return errCannotStartUnattendedSession
	}

	// Start a non-interactive session (TTY attached). Close the session if an error
	// occurs, otherwise it will be closed by the callee.
	scx.setSession(sess, channel)

	err = sess.startExec(ctx, channel, scx)
	if err != nil {
		sess.Close()
		return trace.Wrap(err)
	}

	return nil
}

func (s *SessionRegistry) ForceTerminate(ctx *ServerContext) error {
	sess := ctx.getSession()
	if sess == nil {
		s.logger.DebugContext(s.Srv.Context(), "Unable to terminate session, no session found in context.")
		return nil
	}

	sess.BroadcastMessage("Forcefully terminating session...")

	// Stop session, it will be cleaned up in the background to ensure
	// the session recording is uploaded.
	sess.Stop()

	return nil
}

// GetTerminalSize fetches the terminal size of an active SSH session.
func (s *SessionRegistry) GetTerminalSize(sessionID string) (*term.Winsize, error) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()

	sess := s.sessions[rsession.ID(sessionID)]
	if sess == nil {
		return nil, trace.NotFound("No session found in context.")
	}

	return sess.term.GetWinSize()
}

func (s *SessionRegistry) isApprovedFileTransfer(scx *ServerContext) (bool, error) {
	// If the TELEPORT_MODERATED_SESSION_ID environment variable was not
	// set, return not approved and no error. This means the file
	// transfer came from a non-moderated session. sessionID will be
	// passed after a moderated session approval process has completed.
	sessID, _ := scx.GetEnv(string(sftp.ModeratedSessionID))
	if sessID == "" {
		return false, nil
	}

	// fetch session from registry with sessionID
	s.sessionsMux.Lock()
	sess := s.sessions[rsession.ID(sessID)]
	s.sessionsMux.Unlock()
	if sess == nil {
		// If they sent a sessionID and it wasn't found, send an actual error
		return false, trace.NotFound("Session not found")
	}

	// acquire the session mutex lock so sess.fileTransferReq doesn't get
	// written while we're reading it
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.fileTransferReq == nil {
		return false, trace.NotFound("Session does not have a pending file transfer request")
	}
	if sess.fileTransferReq.Requester != scx.Identity.TeleportUser {
		// to be safe deny and remove the pending request if the user
		// doesn't match what we expect
		req := sess.fileTransferReq
		sess.fileTransferReq = nil

		sess.BroadcastMessage("file transfer request %s denied due to %s attempting to transfer files", req.ID, scx.Identity.TeleportUser)
		_ = s.notifyFileTransferRequestUnderLock(req, FileTransferDenied, scx)

		return false, trace.AccessDenied("Teleport user does not match original requester")
	}

	approved, err := sess.checkIfFileTransferApproved(sess.fileTransferReq)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if approved {
		scx.setApprovedFileTransferRequest(sess.fileTransferReq)
		sess.fileTransferReq = nil
	}

	return approved, nil
}

// FileTransferRequestEvent is an event used to Notify party members during File Transfer Request approval process
type FileTransferRequestEvent string

const (
	// FileTransferUpdate is used when a file transfer request is created or updated.
	// An update will happen if a file transfer request was approved but the policy still isn't fulfilled
	FileTransferUpdate FileTransferRequestEvent = "file_transfer_request"
	// FileTransferApproved is used when a file transfer request has received an approval decision
	// and the policy is fulfilled. This lets the client know that the file transfer is ready to download/upload
	// and be removed from any pending state.
	FileTransferApproved FileTransferRequestEvent = "file_transfer_request_approve"
	// FileTransferDenied is used when a file transfer request is denied. This lets the client know to remove
	// this file transfer from any pending state.
	FileTransferDenied FileTransferRequestEvent = "file_transfer_request_deny"
)

// notifyFileTransferRequestUnderLock is called to notify all members of a party that a file transfer request has been created/approved/denied.
// The notification is a global ssh request and requires the client to update its UI state accordingly.
func (s *SessionRegistry) notifyFileTransferRequestUnderLock(req *FileTransferRequest, res FileTransferRequestEvent, scx *ServerContext) error {
	session := scx.getSession()
	if session == nil {
		s.logger.DebugContext(
			s.Srv.Context(), "Unable to notify event, no session found in context.",
			"event", res,
		)
		return trace.NotFound("no session found in context")
	}

	fileTransferEvent := &apievents.FileTransferRequestEvent{
		Metadata: apievents.Metadata{
			Type:        string(res),
			ClusterName: scx.ClusterName,
		},
		SessionMetadata: session.scx.GetSessionMetadata(),
		RequestID:       req.ID,
		Requester:       req.Requester,
		Location:        req.Location,
		Filename:        req.Filename,
		Download:        req.Download,
		Approvers:       make([]string, 0),
	}

	for _, approver := range req.approvers {
		fileTransferEvent.Approvers = append(fileTransferEvent.Approvers, approver.user)
	}

	eventPayload, err := json.Marshal(fileTransferEvent)
	if err != nil {
		s.logger.WarnContext(
			s.Srv.Context(), "Unable to marshal event.",
			"event", res,
			"error", err,
		)
		return trace.Wrap(err)
	}

	for _, p := range session.parties {
		// Send the message as a global request.
		_, _, err = p.sconn.SendRequest(teleport.SessionEvent, false, eventPayload)
		if err != nil {
			s.logger.WarnContext(
				s.Srv.Context(), "Unable to send event to session party.",
				"event", res,
				"error", err,
				"party", p,
			)
			continue
		}
		s.logger.DebugContext(
			s.Srv.Context(), "Sent event to session party.",
			"event", res,
			"party", p,
		)
	}

	return nil
}

// NotifyWinChange is called to notify all members in the party that the PTY
// size has changed. The notification is sent as a global SSH request and it
// is the responsibility of the client to update it's window size upon receipt.
func (s *SessionRegistry) NotifyWinChange(ctx context.Context, params rsession.TerminalParams, scx *ServerContext) error {
	session := scx.getSession()
	if session == nil {
		s.logger.DebugContext(ctx, "Unable to update window size, no session found in context.")
		return nil
	}

	// Build the resize event.
	resizeEvent, err := session.Recorder().PrepareSessionEvent(&apievents.Resize{
		Metadata: apievents.Metadata{
			Type:        events.ResizeEvent,
			Code:        events.TerminalResizeCode,
			ClusterName: scx.ClusterName,
		},
		ServerMetadata:  session.serverMeta,
		SessionMetadata: session.scx.GetSessionMetadata(),
		UserMetadata:    scx.Identity.GetUserMetadata(),
		TerminalSize:    params.Serialize(),
	})
	if err == nil {
		// Report the updated window size to the session stream (this is so the sessions
		// can be replayed correctly).
		if err := session.recordEvent(s.Srv.Context(), resizeEvent); err != nil {
			s.logger.WarnContext(ctx, "Failed to record resize session event.", "error", err)
		}
	} else {
		s.logger.WarnContext(ctx, "Failed to set up resize session event - event will not be recorded.", "error", err)
	}

	// Update the size of the server side PTY.
	err = session.term.SetWinSize(ctx, params)
	if err != nil {
		return trace.Wrap(err)
	}

	// If sessions are being recorded at the proxy, sessions can not be shared.
	// In that situation, PTY size information does not need to be propagated
	// back to all clients and we can return right away.
	if services.IsRecordAtProxy(scx.SessionRecordingConfig.GetMode()) {
		return nil
	}

	// Notify all members of the party (except originator) that the size of the
	// window has changed so the client can update it's own local PTY. Note that
	// OpenSSH clients will ignore this and not update their own local PTY.
	for _, p := range session.getParties() {
		// Don't send the window change notification back to the originator.
		if p.ctx.ID() == scx.ID() {
			continue
		}

		eventPayload, err := json.Marshal(resizeEvent.GetAuditEvent())
		if err != nil {
			s.logger.WarnContext(ctx, "Unable to marshal resize event for session party.", "error", err, "party", p)
			continue
		}

		// Send the message as a global request.
		_, _, err = p.sconn.SendRequest(teleport.SessionEvent, false, eventPayload)
		if err != nil {
			s.logger.WarnContext(ctx, "Unable to send resize event to session party.", "error", err, "party", p)
			continue
		}
		s.logger.DebugContext(ctx, "Sent resize event to session party.", "event", params, "party", p)
	}

	return nil
}

func (s *SessionRegistry) broadcastResult(sid rsession.ID, r ExecResult) error {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()

	sess, found := s.findSessionLocked(sid)
	if !found {
		return trace.NotFound("session %v not found", sid)
	}
	sess.broadcastResult(r)
	return nil
}

// SessionAccessEvaluator is the interface that defines criteria needed to be met
// in order to start and join sessions.
type SessionAccessEvaluator interface {
	IsModerated() bool
	FulfilledFor(participants []auth.SessionAccessContext) (bool, auth.PolicyOptions, error)
	PrettyRequirementsList() string
	CanJoin(user auth.SessionAccessContext) []types.SessionParticipantMode
}

// session struct describes an active (in progress) SSH session. These sessions
// are managed by 'SessionRegistry' containers which are attached to SSH servers.
type session struct {
	mu sync.RWMutex

	// logger holds the logger for this session.
	logger *slog.Logger

	// session ID. unique GUID, this is what people use to "join" sessions
	id rsession.ID

	// parent session container
	registry *SessionRegistry

	// parties is the set of current connected clients/users. This map may grow
	// and shrink as members join and leave the session.
	parties map[rsession.ID]*party

	// participants is the set of users that have joined this session. Users are
	// never removed from this map as it's used to report the full list of
	// participants at the end of a session.
	participants map[rsession.ID]*party

	// fileTransferReq a pending file transfer request for this session.
	// If the request is denied or approved it should be set to nil to
	// prevent its reuse.
	fileTransferReq *FileTransferRequest

	io       *TermManager
	inWriter io.WriteCloser

	term Terminal

	// stopC channel is used to kill all goroutines owned
	// by the session
	stopC chan struct{}

	// startTime is the time when this session was created.
	startTime time.Time

	// login stores the login of the initial session creator
	login string

	recorder   events.SessionPreparerRecorder
	recorderMu sync.RWMutex

	emitter apievents.Emitter

	// hasEnhancedRecording returns true if this session has enhanced session
	// recording events associated.
	hasEnhancedRecording bool

	// serverCtx is used to control clean up of internal resources
	serverCtx context.Context

	access SessionAccessEvaluator

	tracker *SessionTracker

	initiator string

	scx *ServerContext

	presenceEnabled bool

	doneCh chan struct{}

	displayParticipantRequirements bool

	// endingContext is the server context which closed this session.
	endingContext *ServerContext

	// lingerAndDieCancel is a context cancel func which will cancel
	// an ongoing lingerAndDie goroutine. This is used by joining parties
	// to cancel the goroutine and prevent the session from closing prematurely.
	lingerAndDieCancel func()

	// serverMeta contains metadata about the target node of this session.
	serverMeta apievents.ServerMetadata

	// started is true after the session start.
	started atomic.Bool
}

type sessionType bool

const (
	sessionTypeInteractive    sessionType = true
	sessionTypeNonInteractive sessionType = false
)

// newSession creates a new session with a given ID within a given context.
func newSession(ctx context.Context, id rsession.ID, r *SessionRegistry, scx *ServerContext, ch ssh.Channel, sessType sessionType) (*session, *party, error) {
	serverSessions.Inc()
	startTime := time.Now().UTC()
	rsess := rsession.Session{
		Kind: types.SSHSessionKind,
		ID:   id,
		TerminalParams: rsession.TerminalParams{
			W: teleport.DefaultTerminalWidth,
			H: teleport.DefaultTerminalHeight,
		},
		Login:          scx.Identity.Login,
		Created:        startTime,
		LastActive:     startTime,
		ServerID:       scx.srv.ID(),
		Namespace:      r.Srv.GetNamespace(),
		ServerHostname: scx.srv.GetInfo().GetHostname(),
		ServerAddr:     scx.ServerConn.LocalAddr().String(),
		ClusterName:    scx.ClusterName,
	}

	term := scx.GetTerm()
	if term != nil {
		winsize, err := term.GetWinSize()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		rsess.TerminalParams.W = int(winsize.Width)
		rsess.TerminalParams.H = int(winsize.Height)
	}

	policySets := scx.Identity.AccessChecker.SessionPolicySets()
	access := auth.NewSessionAccessEvaluator(policySets, types.SSHSessionKind, scx.Identity.TeleportUser)
	sess := &session{
		logger: slog.With(
			teleport.ComponentKey, teleport.Component(teleport.ComponentSession, r.Srv.Component()),
			"session_id", id,
		),
		id:                             id,
		registry:                       r,
		parties:                        make(map[rsession.ID]*party),
		participants:                   make(map[rsession.ID]*party),
		login:                          scx.Identity.Login,
		stopC:                          make(chan struct{}),
		startTime:                      startTime,
		emitter:                        scx.srv,
		serverCtx:                      scx.srv.Context(),
		access:                         &access,
		scx:                            scx,
		presenceEnabled:                scx.Identity.UnmappedIdentity.MFAVerified != "",
		io:                             NewTermManager(),
		doneCh:                         make(chan struct{}),
		initiator:                      scx.Identity.TeleportUser,
		displayParticipantRequirements: utils.AsBool(scx.env[teleport.EnvSSHSessionDisplayParticipantRequirements]),
		serverMeta:                     scx.srv.TargetMetadata(),
	}

	sess.io.OnWriteError = sess.onWriteError

	go func() {
		if _, open := <-sess.io.TerminateNotifier(); open {
			err := sess.registry.ForceTerminate(sess.scx)
			if err != nil {
				sess.logger.ErrorContext(sess.serverCtx, "Failed to terminate session.", "error", err)
			}
		}
	}()

	// create a new "party" (connected client) and launch/join the session.
	p := newParty(sess, types.SessionPeerMode, ch, scx)
	sess.parties[p.id] = p
	sess.participants[p.id] = p

	var err error
	if err = sess.trackSession(ctx, scx.Identity.TeleportUser, policySets, p, sessType); err != nil {
		if trace.IsNotImplemented(err) {
			return nil, nil, trace.NotImplemented("Attempted to use Moderated Sessions with an Auth Server below the minimum version of 9.0.0.")
		}
		return nil, nil, trace.Wrap(err)
	}

	sess.recorder, err = newRecorder(sess, scx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return sess, p, nil
}

// ID returns a string representation of the session ID.
func (s *session) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id.String()
}

// PID returns the PID of the Teleport process under which the shell is running.
func (s *session) PID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.term.PID()
}

// Recorder returns a SessionRecorder which can be used to record session
// events.
func (s *session) Recorder() events.SessionPreparerRecorder {
	s.recorderMu.RLock()
	defer s.recorderMu.RUnlock()
	return s.recorder
}

func (s *session) setRecorder(rec events.SessionPreparerRecorder) {
	s.recorderMu.Lock()
	defer s.recorderMu.Unlock()
	s.recorder = rec
}

// Stop ends the active session and forces all clients to disconnect.
// This will trigger background goroutines to complete session cleanup.
func (s *session) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.stopC:
		return
	default:
		close(s.stopC)
	}

	s.BroadcastMessage("Stopping session...")
	s.logger.InfoContext(s.serverCtx, "Stopping session.")

	// Close io copy loops
	if s.inWriter != nil {
		if err := s.inWriter.Close(); err != nil {
			s.logger.DebugContext(s.serverCtx, "Failed to close session writer.", "error", err)
		}
	}
	s.io.Close()

	// Make sure that the terminal has been closed
	s.haltTerminal()

	// Close session tracker and mark it as terminated
	if err := s.tracker.Close(s.serverCtx); err != nil {
		s.logger.DebugContext(s.serverCtx, "Failed to close session tracker.", "error", err)
	}
}

// haltTerminal closes the terminal. Then is tried to terminate the terminal in a graceful way
// and kill by sending SIGKILL if the graceful termination fails.
func (s *session) haltTerminal() {
	if s.term == nil {
		return
	}

	if err := s.term.Close(); err != nil {
		s.logger.DebugContext(s.serverCtx, "Failed to close the shell.", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.term.KillUnderlyingShell(ctx); err != nil {
		s.logger.DebugContext(s.serverCtx, "Failed to terminate the shell.", "error", err)
	} else {
		// Return before we send SIGKILL to the child process, as doing that
		// could interrupt the "graceful shutdown" process.
		return
	}

	if err := s.term.Kill(context.TODO()); err != nil {
		s.logger.DebugContext(s.serverCtx, "Failed to kill the shell.", "error", err)
	}
}

// Close ends the active session and frees all resources. This should only be called
// by the creator of the session, other closers should use Stop instead. Calling this
// prematurely can result in missing audit events, session recordings, and other
// unexpected errors.
func (s *session) Close() error {
	s.BroadcastMessage("Closing session...")
	s.logger.InfoContext(s.serverCtx, "Closing session.")

	// Remove session parties and close client connections. Since terminals
	// might await for all the parties to be released, we must close them first.
	// Closing the parties will cause their SSH channel to be closed, meaning
	// any goroutine reading from it will be released.
	for _, p := range s.getParties() {
		p.Close()
	}

	s.Stop()
	serverSessions.Dec()
	s.registry.removeSession(s)

	// Complete the session recording
	if recorder := s.Recorder(); recorder != nil {
		if err := recorder.Complete(s.serverCtx); err != nil {
			s.logger.WarnContext(s.serverCtx, "Failed to close recorder.", "error", err)
		}
	}

	return nil
}

func (s *session) BroadcastMessage(format string, args ...interface{}) {
	if s.access.IsModerated() && !services.IsRecordAtProxy(s.scx.SessionRecordingConfig.GetMode()) {
		s.io.BroadcastMessage(fmt.Sprintf(format, args...))
	}
}

// BroadcastSystemMessage sends a message to all parties.
func (s *session) BroadcastSystemMessage(format string, args ...interface{}) {
	s.io.BroadcastMessage(fmt.Sprintf(format, args...))
}

// emitSessionStartEvent emits a session start event.
func (s *session) emitSessionStartEvent(ctx *ServerContext) {
	var initialCommand []string
	if execRequest, err := ctx.GetExecRequest(); err == nil {
		initialCommand = []string{execRequest.GetCommand()}
	}

	sessionStartEvent := &apievents.SessionStart{
		Metadata: apievents.Metadata{
			Type:        events.SessionStartEvent,
			Code:        events.SessionStartCode,
			ClusterName: ctx.ClusterName,
			ID:          uuid.New().String(),
		},
		ServerMetadata:  s.serverMeta,
		SessionMetadata: s.scx.GetSessionMetadata(),
		UserMetadata:    ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
			Protocol:   events.EventProtocolSSH,
		},
		SessionRecording: s.sessionRecordingMode(),
		InitialCommand:   initialCommand,
		Reason:           s.scx.env[teleport.EnvSSHSessionReason],
	}

	if invitedUsers := s.scx.env[teleport.EnvSSHSessionInvited]; invitedUsers != "" {
		if err := json.Unmarshal([]byte(invitedUsers), &sessionStartEvent.Invited); err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Failed to parse invited users", "error", err)
		}
	}

	if s.term != nil {
		params := s.term.GetTerminalParams()
		sessionStartEvent.TerminalSize = params.Serialize()
	}

	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		sessionStartEvent.ConnectionMetadata.LocalAddr = ctx.ServerConn.LocalAddr().String()
	}

	preparedEvent, err := s.Recorder().PrepareSessionEvent(sessionStartEvent)
	if err != nil {
		s.logger.WarnContext(ctx.srv.Context(), "Failed to set up session start event - event will not be recorded", "error", err)
		return
	}
	if err := s.recordEvent(ctx.srv.Context(), preparedEvent); err != nil {
		s.logger.WarnContext(ctx.srv.Context(), "Failed to record session start event.", "error", err)
	}
	if err := s.emitAuditEvent(ctx.srv.Context(), preparedEvent.GetAuditEvent()); err != nil {
		s.logger.WarnContext(ctx.srv.Context(), "Failed to emit session start event.", "error", err)
	}
}

// emitSessionJoinEvent emits a session join event to both the Audit Log as
// well as sending a "x-teleport-event" global request on the SSH connection.
// Must be called under session Lock.
func (s *session) emitSessionJoinEvent(ctx *ServerContext) {
	sessionJoinEvent := &apievents.SessionJoin{
		Metadata: apievents.Metadata{
			Type:        events.SessionJoinEvent,
			Code:        events.SessionJoinCode,
			ClusterName: ctx.ClusterName,
		},
		ServerMetadata:  s.serverMeta,
		SessionMetadata: ctx.GetSessionMetadata(),
		UserMetadata:    ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
		},
	}
	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		sessionJoinEvent.ConnectionMetadata.LocalAddr = ctx.ServerConn.LocalAddr().String()
	}

	var notifyPartyPayload []byte
	preparedEvent, err := s.Recorder().PrepareSessionEvent(sessionJoinEvent)
	if err == nil {
		// Try marshaling the event prior to emitting it to prevent races since
		// the audit/recording machinery might try to set some fields while the
		// marshal is underway.
		if eventPayload, err := json.Marshal(preparedEvent); err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Unable to marshal session join event.", "error", err)
		} else {
			notifyPartyPayload = eventPayload
		}

		if err := s.recordEvent(ctx.srv.Context(), preparedEvent); err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Failed to record session join event.", "error", err)
		}
		if err := s.emitAuditEvent(ctx.srv.Context(), preparedEvent.GetAuditEvent()); err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Failed to emit session join event.", "error", err)
		}
	} else {
		s.logger.WarnContext(ctx.srv.Context(), "Failed to set up session join event - event will not be recorded", "error", err)
	}

	// Notify all members of the party that a new member has joined over the
	// "x-teleport-event" channel.
	for _, p := range s.getParties() {
		if len(notifyPartyPayload) == 0 {
			s.logger.WarnContext(ctx.srv.Context(), "No session join event to send to party.", "party", p)
			continue
		}

		payload := make([]byte, len(notifyPartyPayload))
		copy(payload, notifyPartyPayload)

		_, _, err = p.sconn.SendRequest(teleport.SessionEvent, false, payload)
		if err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Unable to send session join event to party.", "party", p, "error", err)
			continue
		}
		s.logger.DebugContext(ctx.srv.Context(), "Sent session join event to party.", "party", p)
	}
}

// emitSessionLeaveEventUnderLock emits a session leave event to both the Audit Log as
// well as sending a "x-teleport-event" global request on the SSH connection.
// Must be called under session Lock.
func (s *session) emitSessionLeaveEventUnderLock(ctx *ServerContext) {
	sessionLeaveEvent := &apievents.SessionLeave{
		Metadata: apievents.Metadata{
			Type:        events.SessionLeaveEvent,
			Code:        events.SessionLeaveCode,
			ClusterName: ctx.ClusterName,
		},
		ServerMetadata:  s.serverMeta,
		SessionMetadata: s.scx.GetSessionMetadata(),
		UserMetadata:    ctx.Identity.GetUserMetadata(),
	}
	preparedEvent, err := s.Recorder().PrepareSessionEvent(sessionLeaveEvent)
	if err == nil {
		if err := s.recordEvent(ctx.srv.Context(), preparedEvent); err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Failed to record session leave event.", "error", err)
		}
		if err := s.emitAuditEvent(ctx.srv.Context(), preparedEvent.GetAuditEvent()); err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Failed to emit session leave event.", "error", err)
		}
	} else {
		s.logger.WarnContext(ctx.srv.Context(), "Failed to set up session leave event - event will not be recorded.", "error", err)
	}

	// Notify all members of the party that a new member has left over the
	// "x-teleport-event" channel.
	for _, p := range s.parties {
		eventPayload, err := utils.FastMarshal(sessionLeaveEvent)
		if err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Unable to marshal session leave event for party.", "error", err, "party", p)
			continue
		}
		_, _, err = p.sconn.SendRequest(teleport.SessionEvent, false, eventPayload)
		if err != nil {
			// The party's connection may already be closed, in which case we expect an EOF
			if !errors.Is(err, io.EOF) {
				s.logger.WarnContext(ctx.srv.Context(), "Unable to send session leave event to party.", "party", p, "error", err)
			}
			continue
		}
		s.logger.DebugContext(ctx.srv.Context(), "Sent session leave event to party.", "party", p)
	}
}

// emitSessionEndEvent emits a session end event.
func (s *session) emitSessionEndEvent() {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := s.scx
	if s.endingContext != nil {
		ctx = s.endingContext
	}

	start, end := s.startTime, time.Now().UTC()
	sessionEndEvent := &apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Type:        events.SessionEndEvent,
			Code:        events.SessionEndCode,
			ClusterName: ctx.ClusterName,
		},
		ServerMetadata:  s.serverMeta,
		SessionMetadata: s.scx.GetSessionMetadata(),
		UserMetadata:    ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
			Protocol:   events.EventProtocolSSH,
		},
		EnhancedRecording: s.hasEnhancedRecording,
		Interactive:       s.term != nil,
		StartTime:         start,
		EndTime:           end,
		SessionRecording:  s.sessionRecordingMode(),
	}

	for _, p := range s.participants {
		sessionEndEvent.Participants = append(sessionEndEvent.Participants, p.user)
	}

	// If there are 0 participants, this is an exec session.
	// Use the user from the session context.
	if len(s.participants) == 0 {
		sessionEndEvent.Participants = []string{s.scx.Identity.TeleportUser}
	}

	preparedEvent, err := s.Recorder().PrepareSessionEvent(sessionEndEvent)
	if err == nil {
		if err := s.recordEvent(ctx.srv.Context(), preparedEvent); err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Failed to record session end event.", "error", err)
		}
		if err := s.emitAuditEvent(ctx.srv.Context(), preparedEvent.GetAuditEvent()); err != nil {
			s.logger.WarnContext(ctx.srv.Context(), "Failed to emit session end event.", "error", err)
		}
	} else {
		s.logger.WarnContext(ctx.srv.Context(), "Failed to set up session end event - event will not be recorded.", "error", err)
	}
}

func (s *session) sessionRecordingMode() string {
	sessionRecMode := s.scx.SessionRecordingConfig.GetMode()
	subKind := s.serverMeta.ServerSubKind

	// agentless connections always record the session at the proxy
	if !services.IsRecordAtProxy(sessionRecMode) && types.IsOpenSSHNodeSubKind(subKind) {
		if services.IsRecordSync(sessionRecMode) {
			sessionRecMode = types.RecordAtProxySync
		} else {
			sessionRecMode = types.RecordAtProxy
		}
	}

	return sessionRecMode
}

func (s *session) setEndingContext(ctx *ServerContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endingContext = ctx
}

func (s *session) setHasEnhancedRecording(val bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasEnhancedRecording = val
}

// launch launches the session.
// Must be called under session Lock.
func (s *session) launch() {
	// Mark the session as started here, as we want to avoid double initialization.
	if s.started.Swap(true) {
		s.logger.DebugContext(s.serverCtx, "Session has already started.")
		return
	}

	s.logger.DebugContext(s.serverCtx, "Launching session.")
	s.BroadcastMessage("Connecting to %v over SSH", s.serverMeta.ServerHostname)

	s.io.On()

	if err := s.tracker.UpdateState(s.serverCtx, types.SessionState_SessionStateRunning); err != nil {
		s.logger.WarnContext(
			s.serverCtx, "Failed to set tracker state.",
			"error", err,
			"state", types.SessionState_SessionStateRunning,
		)
	}

	// If the identity is verified with an MFA device, we enabled MFA-based presence for the session.
	if s.presenceEnabled {
		go func() {
			ticker := s.registry.clock.NewTicker(PresenceVerifyInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.Chan():
					err := s.checkPresence(s.serverCtx)
					if err != nil {
						s.logger.ErrorContext(
							s.serverCtx, "Failed to check presence, terminating session as a security measure.",
						)
						s.Stop()
					}
				case <-s.stopC:
					return
				}
			}
		}()
	}

	// copy everything from the pty to the writer. this lets us capture all input
	// and output of the session (because input is echoed to stdout in the pty).
	// the writer contains multiple writers: the session logger and a direct
	// connection to members of the "party" (other people in the session).
	s.term.AddParty(1)
	go func() {
		defer s.term.AddParty(-1)

		// once everything has been copied, notify the goroutine below. if this code
		// is running in a teleport node, when the exec.Cmd is done it will close
		// the PTY, allowing io.Copy to return. if this is a teleport forwarding
		// node, when the remote side closes the channel (which is what s.term.PTY()
		// returns) io.Copy will return.
		defer close(s.doneCh)

		_, err := io.Copy(s.io, s.term.PTY())
		s.logger.DebugContext(
			s.serverCtx, "Copying from PTY to writer completed with error.",
			"error", err,
		)
	}()

	s.term.AddParty(1)
	go func() {
		defer s.term.AddParty(-1)

		_, err := io.Copy(s.term.PTY(), s.io)
		s.logger.DebugContext(
			s.serverCtx, "Copying from reader to PTY completed with error.",
			"error", err,
		)
	}()
}

// startInteractive starts a new interactive process (or a shell) in the
// current session.
func (s *session) startInteractive(ctx context.Context, scx *ServerContext, p *party) error {
	s.mu.Lock()
	canStart, _, err := s.checkIfStartUnderLock()
	s.mu.Unlock()
	if err != nil {
		return trace.Wrap(err)
	}
	if !canStart && services.IsRecordAtProxy(p.ctx.SessionRecordingConfig.GetMode()) {
		go s.Stop()
		return trace.AccessDenied("session requires additional moderation but is in proxy-record mode")
	}

	inReader, inWriter := io.Pipe()

	s.mu.Lock()
	s.inWriter = inWriter
	s.mu.Unlock()

	s.io.AddReader("reader", inReader)
	s.io.AddWriter(sessionRecorderID, utils.WriteCloserWithContext(scx.srv.Context(), s.Recorder()))
	s.BroadcastMessage("Creating session with ID: %v", s.id)

	if err := s.startTerminal(ctx, scx); err != nil {
		return trace.Wrap(err)
	}

	// Emit a session.start event for the interactive session.
	s.emitSessionStartEvent(scx)

	if err := s.addParty(p, types.SessionPeerMode); err != nil {
		return trace.Wrap(err)
	}

	// Open a BPF recording session. If BPF was not configured, not available,
	// or running in a recording proxy, OpenSession is a NOP.
	sessionContext := &bpf.SessionContext{
		Context:        scx.srv.Context(),
		PID:            s.term.PID(),
		Emitter:        s.emitter,
		Namespace:      scx.srv.GetNamespace(),
		SessionID:      s.id.String(),
		ServerID:       scx.srv.HostUUID(),
		ServerHostname: scx.srv.GetInfo().GetHostname(),
		Login:          scx.Identity.Login,
		User:           scx.Identity.TeleportUser,
		Events:         scx.Identity.AccessChecker.EnhancedRecordingSet(),
	}

	bpfService := scx.srv.GetBPF()

	// Only wait for the child to be "ready" if BPF is enabled. This is required
	// because PAM might inadvertently move the child process to another cgroup
	// by invoking systemd. If this happens, then the cgroup filter used by BPF
	// will be looking for events in the wrong cgroup and no events will be captured.
	// However, unconditionally waiting for the child to be ready results in PAM
	// deadlocking because stdin/stdout/stderr which it uses to relay details from
	// PAM auth modules are not properly copied until _after_ the shell request is
	// replied to.
	if bpfService.Enabled() {
		if err := s.term.WaitForChild(); err != nil {
			s.logger.ErrorContext(ctx, "Child process never became ready.", "error", err)
			return trace.Wrap(err)
		}
	} else {
		// Clean up the read half of the pipe, and set it to nil so that when the
		// ServerContext is closed it doesn't attempt to a second close.
		if err := s.scx.readyr.Close(); err != nil {
			s.logger.ErrorContext(ctx, "Failed closing child ready pipe", "error", err)
		}
		s.scx.readyr = nil
	}

	if cgroupID, err := bpfService.OpenSession(sessionContext); err != nil {
		s.logger.ErrorContext(ctx, "Failed to open enhanced recording (interactive) session.", "error", err)
		return trace.Wrap(err)
	} else if cgroupID > 0 {
		// If a cgroup ID was assigned then enhanced session recording was enabled.
		s.setHasEnhancedRecording(true)
		go func() {
			// Close the BPF recording session once the session is closed
			<-s.stopC
			if err := bpfService.CloseSession(sessionContext); err != nil {
				s.logger.ErrorContext(ctx, "Failed to close enhanced recording (interactive) session.", "error", err)
			}
		}()
	}

	s.logger.DebugContext(ctx, "Waiting for continue signal.")

	// Process has been placed in a cgroup, continue execution.
	s.term.Continue()

	s.logger.DebugContext(ctx, "Got continue signal.")

	// wait for exec.Cmd (or receipt of "exit-status" for a forwarding node),
	// once it is received wait for the io.Copy above to finish, then broadcast
	// the "exit-status" to the client.
	go func() {
		result, err := s.term.Wait()
		if err != nil {
			s.logger.ErrorContext(ctx, "Received error waiting for the interactive session to finish.", "error", err)
		}

		// wait for copying from the pty to be complete or a timeout before
		// broadcasting the result (which will close the pty) if it has not been
		// closed already.
		select {
		case <-time.After(defaults.WaitCopyTimeout):
			s.logger.DebugContext(ctx, "Timed out waiting for PTY copy to finish, session data may be missing.")
		case <-s.doneCh:
		}

		if result != nil {
			if err := s.registry.broadcastResult(s.id, *result); err != nil {
				s.logger.WarnContext(ctx, "Failed to broadcast session result.", "error", err)
			}
		}

		if execRequest, err := scx.GetExecRequest(); err == nil && execRequest.GetCommand() != "" {
			emitExecAuditEvent(scx, execRequest.GetCommand(), err)
		}

		s.emitSessionEndEvent()
		if err := s.Close(); err != nil {
			s.logger.WarnContext(ctx, "Failed to close session.", "error", err)
		}
	}()

	return nil
}

func (s *session) startTerminal(ctx context.Context, scx *ServerContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// allocate a terminal or take the one previously allocated via a
	// separate "allocate TTY" SSH request
	var err error
	if s.term = scx.GetTerm(); s.term != nil {
		scx.SetTerm(nil)
	} else if s.term, err = NewTerminal(scx); err != nil {
		s.logger.InfoContext(ctx, "Unable to allocate new terminal.", "error", err)
		return trace.Wrap(err)
	}

	if err := s.term.Run(ctx); err != nil {
		s.logger.ErrorContext(ctx, "Unable to run shell command.", "error", err)
		return trace.ConvertSystemError(err)
	}

	return nil
}

// newRecorder creates a new [events.SessionPreparerRecorder] to be used as the recorder
// of the passed in session.
func newRecorder(s *session, ctx *ServerContext) (events.SessionPreparerRecorder, error) {
	// Nodes discard events in cases when proxies are already recording them.
	if s.registry.Srv.Component() == teleport.ComponentNode &&
		services.IsRecordAtProxy(ctx.SessionRecordingConfig.GetMode()) {
		s.logger.DebugContext(s.serverCtx, "Session will be recorded at proxy.")
		return events.WithNoOpPreparer(events.NewDiscardRecorder()), nil
	}

	// Don't record non-interactive sessions when enhanced recording is disabled.
	if ctx.GetTerm() == nil && !ctx.srv.GetBPF().Enabled() {
		return events.WithNoOpPreparer(events.NewDiscardRecorder()), nil
	}

	rec, err := recorder.New(recorder.Config{
		SessionID:    s.id,
		ServerID:     s.serverMeta.ServerID,
		Namespace:    s.serverMeta.ServerNamespace,
		Clock:        s.registry.clock,
		ClusterName:  ctx.ClusterName,
		RecordingCfg: ctx.SessionRecordingConfig,
		SyncStreamer: ctx.srv,
		DataDir:      ctx.srv.GetDataDir(),
		Component:    teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
		// Session stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context: ctx.srv.Context(),
	})
	if err != nil {
		switch ctx.Identity.AccessChecker.SessionRecordingMode(constants.SessionRecordingServiceSSH) {
		case constants.SessionRecordingModeBestEffort:
			s.logger.WarnContext(
				s.serverCtx, "Failed to initialize session recording, disabling it for this session.",
				"error", err,
			)

			s.BroadcastSystemMessage(sessionRecordingWarningMessage)
			return events.WithNoOpPreparer(events.NewDiscardRecorder()), nil
		}

		return nil, trace.ConnectionProblem(err, sessionRecordingErrorMessage)
	}

	return rec, nil
}

func (s *session) startExec(ctx context.Context, channel ssh.Channel, scx *ServerContext) error {
	// Emit a session.start event for the exec session.
	s.emitSessionStartEvent(scx)

	execRequest, err := scx.GetExecRequest()
	if err != nil {
		return trace.Wrap(err)
	}

	// Start execution. If the program failed to start, send that result back.
	// Note this is a partial start. Teleport will have re-exec'ed itself and
	// wait until it's been placed in a cgroup and told to continue.
	result, err := execRequest.Start(ctx, channel)
	if err != nil {
		return trace.Wrap(err)
	}
	if result != nil {
		s.logger.DebugContext(
			ctx, "Exec request completed.",
			"request", execRequest,
			"result", result,
		)
		scx.SendExecResult(*result)
	}

	// Open a BPF recording session. If BPF was not configured, not available,
	// or running in a recording proxy, OpenSession is a NOP.
	sessionContext := &bpf.SessionContext{
		Context:        scx.srv.Context(),
		PID:            scx.execRequest.PID(),
		Emitter:        s.emitter,
		Namespace:      scx.srv.GetNamespace(),
		SessionID:      string(s.id),
		ServerID:       scx.srv.HostUUID(),
		ServerHostname: scx.srv.GetInfo().GetHostname(),
		Login:          scx.Identity.Login,
		User:           scx.Identity.TeleportUser,
		Events:         scx.Identity.AccessChecker.EnhancedRecordingSet(),
	}

	if err := execRequest.WaitForChild(); err != nil {
		s.logger.ErrorContext(ctx, "Child process never became ready.", "error", err)
		return trace.Wrap(err)
	}

	cgroupID, err := scx.srv.GetBPF().OpenSession(sessionContext)
	if err != nil {
		s.logger.ErrorContext(
			ctx, "Failed to open enhanced recording (exec) session.",
			"command", execRequest.GetCommand(),
			"error", err,
		)
		return trace.Wrap(err)
	}

	// If a cgroup ID was assigned then enhanced session recording was enabled.
	if cgroupID > 0 {
		s.setHasEnhancedRecording(true)
	}

	// Process has been placed in a cgroup, continue execution.
	execRequest.Continue()

	// Process is running, wait for it to stop.
	go func() {
		result = execRequest.Wait()
		if result != nil {
			scx.SendExecResult(*result)
		}

		// Wait a little bit to let all events filter through before closing the
		// BPF session so everything can be recorded.
		time.Sleep(2 * time.Second)

		// Close the BPF recording session. If BPF was not configured, not available,
		// or running in a recording proxy, this is simply a NOP.
		err = scx.srv.GetBPF().CloseSession(sessionContext)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to close enhanced recording (exec) session.", "error", err)
		}

		s.emitSessionEndEvent()
		s.Close()

		s.io.Close()
		close(s.doneCh)
	}()

	return nil
}

func (s *session) broadcastResult(r ExecResult) {
	payload := ssh.Marshal(struct{ C uint32 }{C: uint32(r.Code)})
	for _, p := range s.getParties() {
		if _, err := p.ch.SendRequest("exit-status", false, payload); err != nil {
			s.logger.InfoContext(
				s.serverCtx, "Failed to send exit status",
				"command", r.Command,
				"error", err,
			)
		}
	}
}

func (s *session) String() string {
	return fmt.Sprintf("session(id=%v, parties=%v)", s.id, len(s.getParties()))
}

// removePartyUnderLock removes the party from the in-memory map that holds all party members
// and closes their underlying ssh channels. This may also trigger the session to end
// if the party is the last in the session or has policies that dictate it to end.
// Must be called under session Lock.
func (s *session) removePartyUnderLock(p *party) error {
	s.logger.InfoContext(s.serverCtx, "Removing party from session.", "party", p)

	// Remove participant from in-memory map of party members.
	delete(s.parties, p.id)

	s.BroadcastMessage("User %v left the session.", p.user)

	// Update session tracker
	s.logger.DebugContext(s.serverCtx, "Removing participant from tracker.", "party", p)
	if err := s.tracker.RemoveParticipant(s.serverCtx, p.id.String()); err != nil {
		return trace.Wrap(err)
	}

	// Remove party for the term writer
	s.io.DeleteWriter(string(p.id))

	// Emit session leave event to both the Audit Log and over the
	// "x-teleport-event" channel in the SSH connection.
	s.emitSessionLeaveEventUnderLock(p.ctx)

	canRun, policyOptions, err := s.checkIfStartUnderLock()
	if err != nil {
		return trace.Wrap(err)
	}

	if !canRun {
		if policyOptions.OnLeaveAction == types.OnSessionLeaveTerminate {
			// Force termination in goroutine to avoid deadlock
			go s.registry.ForceTerminate(s.scx)
			return nil
		}

		// pause session and wait for another party to resume
		s.io.Off()
		s.BroadcastMessage("Session paused, Waiting for required participants...")
		if err := s.tracker.UpdateState(s.serverCtx, types.SessionState_SessionStatePending); err != nil {
			s.logger.WarnContext(
				s.serverCtx, "Failed to set tracker state.",
				"error", err,
				"state", types.SessionState_SessionStatePending,
			)
		}

		go func() {
			if state := s.tracker.WaitForStateUpdate(types.SessionState_SessionStatePending); state == types.SessionState_SessionStateRunning {
				s.BroadcastMessage("Resuming session...")
				s.io.On()
			}
		}()
	}

	// If the leaving party was the last one in the session, start the lingerAndDie
	// goroutine. Parties that join during the linger duration will cancel the
	// goroutine to prevent the session from ending with active parties.
	if len(s.parties) == 0 && !s.isStopped() {
		ctx, cancel := context.WithCancel(s.serverCtx)
		s.lingerAndDieCancel = cancel
		go s.lingerAndDie(ctx, p)
	}

	return nil
}

// isStopped does not need to be called under sessionLock
func (s *session) isStopped() bool {
	select {
	case <-s.stopC:
		return true
	default:
		return false
	}
}

// lingerAndDie will let the party-less session linger for a short
// duration, and then die if no parties have joined.
func (s *session) lingerAndDie(ctx context.Context, party *party) {
	s.logger.DebugContext(ctx, "Session has no active party members.")

	select {
	case <-s.registry.clock.After(defaults.SessionIdlePeriod):
		s.logger.InfoContext(ctx, "Session will be garbage collected.")

		// set closing context to the leaving party to show who ended the session.
		s.setEndingContext(party.ctx)

		// Stop the session, and let the background processes
		// complete cleanup and close the session.
		s.Stop()
	case <-ctx.Done():
		s.logger.InfoContext(ctx, "Session has become active again.")
		return
	case <-s.stopC:
		return
	}
}

func (s *session) checkPresence(ctx context.Context) error {
	// We cannot check presence on the local tracker as that will not
	// be updated in response to parties performing their presence
	// checks. To prevent the stale version of the session tracker from
	// terminating a session we must get the session tracker from Auth.
	tracker, err := s.registry.SessionTrackerService.GetSessionTracker(ctx, s.tracker.tracker.GetSessionID())
	if err != nil {
		return trace.Wrap(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, participant := range tracker.GetParticipants() {
		if participant.User == s.initiator {
			continue
		}

		if participant.Mode == string(types.SessionModeratorMode) && s.registry.clock.Now().UTC().After(participant.LastActive.Add(PresenceMaxDifference)) {
			s.logger.WarnContext(
				ctx, "Participant is not active, kicking.",
				"participant", participant.ID,
			)
			if party := s.parties[rsession.ID(participant.ID)]; party != nil {
				if err := party.closeUnderSessionLock(); err != nil {
					s.logger.ErrorContext(
						ctx, "Failed to remove party.",
						"party", party, "error", err)
				}
			}
		}
	}

	return nil
}

// FileTransferRequest is a request to upload or download a file from a node.
type FileTransferRequest struct {
	// ID is a UUID that uniquely identifies a file transfer request
	// and is unlikely to collide with another file transfer request
	ID string
	// Requester is the Teleport User that requested the file transfer
	Requester string
	// Download is true if the request is a download, false if its an upload
	Download bool
	// Filename is the name of the file to upload.
	Filename string
	// Location of the requested download or where a file will be uploaded
	Location string
	// approvers is a list of participants of moderator or peer type that have approved the request
	approvers map[string]*party
}

func (s *session) checkIfFileTransferApproved(req *FileTransferRequest) (bool, error) {
	var participants []auth.SessionAccessContext

	for _, party := range req.approvers {
		if party.ctx.Identity.TeleportUser == s.initiator {
			continue
		}

		participants = append(participants, auth.SessionAccessContext{
			Username: party.ctx.Identity.TeleportUser,
			Roles:    party.ctx.Identity.AccessChecker.Roles(),
			Mode:     party.mode,
		})
	}

	isApproved, _, err := s.access.FulfilledFor(participants)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return isApproved, nil
}

// newFileTransferRequest takes FileTransferParams and creates a new fileTransferRequest struct
func (s *session) newFileTransferRequest(params *rsession.FileTransferRequestParams) (*FileTransferRequest, error) {
	location, err := s.expandFileTransferRequestPath(params.Location)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := FileTransferRequest{
		ID:        uuid.New().String(),
		Requester: params.Requester,
		Location:  location,
		Filename:  params.Filename,
		Download:  params.Download,
		approvers: make(map[string]*party),
	}

	return &req, nil
}

func (s *session) expandFileTransferRequestPath(p string) (string, error) {
	expanded := filepath.Clean(p)
	dir := filepath.Dir(expanded)

	var tildePrefixed bool
	var noBaseDir bool
	if dir == "~" {
		tildePrefixed = true
	} else if dir == "." {
		noBaseDir = true
	}

	if tildePrefixed || noBaseDir {
		localUser, err := user.Lookup(s.login)
		if err != nil {
			return "", trace.Wrap(err)
		}

		exists, err := CheckHomeDir(localUser)
		if err != nil {
			return "", trace.Wrap(err)
		}
		homeDir := localUser.HomeDir
		if !exists {
			homeDir = string(os.PathSeparator)
		}

		if tildePrefixed {
			// expand home dir to make an absolute path
			expanded = filepath.Join(homeDir, expanded[2:])
		} else {
			// if no directories are specified SFTP will assume the file
			// to be in the user's home dir
			expanded = filepath.Join(homeDir, expanded)
		}
	}

	return expanded, nil
}

// addFileTransferRequest will create a new file transfer request and add it to the current session's fileTransferRequests map
// and broadcast the appropriate string to the session.
func (s *session) addFileTransferRequest(params *rsession.FileTransferRequestParams, scx *ServerContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fileTransferReq != nil {
		return trace.AlreadyExists("a file transfer request already exists for this session")
	}
	if !params.Download && params.Filename == "" {
		return trace.BadParameter("no source file is set for the upload")
	}

	req, err := s.newFileTransferRequest(params)
	if err != nil {
		return trace.Wrap(err)
	}
	s.fileTransferReq = req

	if params.Download {
		s.BroadcastMessage("User %s would like to download: %s", params.Requester, params.Location)
	} else {
		s.BroadcastMessage("User %s would like to upload %s to: %s", params.Requester, params.Filename, params.Location)
	}
	err = s.registry.notifyFileTransferRequestUnderLock(s.fileTransferReq, FileTransferUpdate, scx)

	return trace.Wrap(err)
}

// approveFileTransferRequest will add the approver to the approvers map of a file transfer request and notify the members
// of the session if the updated approvers map would fulfill the moderated policy.
func (s *session) approveFileTransferRequest(params *rsession.FileTransferDecisionParams, scx *ServerContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fileTransferReq == nil {
		return trace.NotFound("File Transfer Request %s not found", params.RequestID)
	}
	if s.fileTransferReq.ID != params.RequestID {
		return trace.BadParameter("current file transfer request is not %s", params.RequestID)
	}

	var approver *party
	for _, p := range s.parties {
		if p.ctx.ID() == scx.ID() {
			approver = p
		}
	}
	if approver == nil {
		return trace.AccessDenied("cannot approve file transfer requests if not in the current moderated session")
	}

	s.fileTransferReq.approvers[approver.user] = approver
	s.BroadcastMessage("%s approved file transfer request %s", scx.Identity.TeleportUser, s.fileTransferReq.ID)

	// check if policy is fulfilled
	approved, err := s.checkIfFileTransferApproved(s.fileTransferReq)
	if err != nil {
		return trace.Wrap(err)
	}

	var eventType FileTransferRequestEvent
	if approved {
		eventType = FileTransferApproved
	} else {
		eventType = FileTransferUpdate
	}
	err = s.registry.notifyFileTransferRequestUnderLock(s.fileTransferReq, eventType, scx)

	return trace.Wrap(err)
}

// denyFileTransferRequest will deny a file transfer request and remove it from the current session's file transfer requests map.
// A file transfer request does not persist after deny, so there is no "denied" state. Deny in this case is synonymous with delete
// with the addition of checking for a valid denier.
func (s *session) denyFileTransferRequest(params *rsession.FileTransferDecisionParams, scx *ServerContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fileTransferReq == nil {
		return trace.NotFound("file transfer request %s not found", params.RequestID)
	}
	if s.fileTransferReq.ID != params.RequestID {
		return trace.BadParameter("current file transfer request is not %s", params.RequestID)
	}

	var denier *party
	for _, p := range s.parties {
		if p.ctx.ID() == scx.ID() {
			denier = p
		}
	}
	if denier == nil {
		return trace.AccessDenied("cannot deny file transfer requests if not in the current moderated session")
	}

	req := s.fileTransferReq
	s.fileTransferReq = nil

	s.BroadcastMessage("%s denied file transfer request %s", scx.Identity.TeleportUser, req.ID)
	err := s.registry.notifyFileTransferRequestUnderLock(req, FileTransferDenied, scx)

	return trace.Wrap(err)
}

// checkIfStartUnderLock determines if any moderation policies associated with
// the session are satisfied.
// Must be called under session Lock.
func (s *session) checkIfStartUnderLock() (bool, auth.PolicyOptions, error) {
	var participants []auth.SessionAccessContext

	for _, party := range s.parties {
		if party.ctx.Identity.TeleportUser == s.initiator {
			continue
		}

		participants = append(participants, auth.SessionAccessContext{
			Username: party.ctx.Identity.TeleportUser,
			Roles:    party.ctx.Identity.AccessChecker.Roles(),
			Mode:     party.mode,
		})
	}

	shouldStart, policyOptions, err := s.access.FulfilledFor(participants)
	if err != nil {
		return false, auth.PolicyOptions{}, trace.Wrap(err)
	}

	return shouldStart, policyOptions, nil
}

// addParty is called when a new party joins the session.
func (s *session) addParty(p *party, mode types.SessionParticipantMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.login != p.login && p.login != teleport.SSHSessionJoinPrincipal {
		return trace.AccessDenied(
			"can't switch users from %v to %v for session %v",
			s.login, p.login, s.id)
	}

	if s.tracker.GetState() == types.SessionState_SessionStateTerminated {
		return trace.AccessDenied("The requested session is not active")
	}

	if len(s.parties) == 0 {
		canStart, _, err := s.checkIfStartUnderLock()
		if err != nil {
			return trace.Wrap(err)
		}

		if !canStart && services.IsRecordAtProxy(p.ctx.SessionRecordingConfig.GetMode()) {
			go s.Stop()
			return trace.AccessDenied("session requires additional moderation but is in proxy-record mode")
		}
	}

	// Cancel lingerAndDie goroutine if one is running.
	if s.lingerAndDieCancel != nil {
		s.lingerAndDieCancel()
		s.lingerAndDieCancel = nil
	}

	// Adds participant to in-memory map of party members.
	s.parties[p.id] = p
	s.participants[p.id] = p
	p.ctx.AddCloser(p)

	// Write last chunk (so the newly joined parties won't stare at a blank screen).
	if _, err := p.Write(s.io.GetRecentHistory()); err != nil {
		return trace.Wrap(err)
	}
	s.BroadcastMessage("User %v joined the session with participant mode: %v.", p.user, p.mode)

	// Register this party as one of the session writers (output will go to it).
	s.io.AddWriter(string(p.id), p)

	// Send the participant mode and controls to the additional participant
	if s.login != p.login {
		err := MsgParticipantCtrls(p.ch, mode)
		if err != nil {
			s.logger.ErrorContext(
				s.serverCtx, "Could not send into message to participant",
				"error", err,
			)
		}
	}

	s.logger.InfoContext(
		s.serverCtx, "New party has joined the session with a participant mode",
		"participant", p.String(),
		"mode", p.mode,
	)

	if mode == types.SessionPeerMode {
		s.term.AddParty(1)

		// This goroutine keeps pumping party's input into the session.
		go func() {
			defer s.term.AddParty(-1)
			_, err := io.Copy(s.inWriter, p)
			s.logger.DebugContext(
				s.serverCtx, "Copying from Party to session writer completed with error.",
				"party", p,
				"error", err,
			)
		}()
	}

	if s.tracker.GetState() == types.SessionState_SessionStatePending {
		canStart, _, err := s.checkIfStartUnderLock()
		if err != nil {
			return trace.Wrap(err)
		}

		switch {
		case canStart && !s.started.Load():
			s.launch()

			return nil
		case canStart:
			// If the session is already running, but the party is a moderator that leaved
			// a session with onLeave=pause and then rejoined, we need to unpause the session.
			// When the moderator leaved the session, the session was paused, and we spawn
			// a goroutine to wait for the moderator to rejoin. If the moderator rejoins
			// before the session ends, we need to unpause the session by updating its state and
			// the goroutine will unblock the s.io terminal.
			// types.SessionState_SessionStatePending marks a session that is waiting for
			// a moderator to rejoin.
			if err := s.tracker.UpdateState(s.serverCtx, types.SessionState_SessionStateRunning); err != nil {
				s.logger.WarnContext(
					s.serverCtx, "Failed to set tracker state.",
					"state", types.SessionState_SessionStateRunning,
				)
			}
		default:
			const base = "Waiting for required participants..."
			if s.displayParticipantRequirements {
				s.BroadcastMessage(base+"\r\n%v", s.access.PrettyRequirementsList())
			} else {
				s.BroadcastMessage(base)
			}
		}
	}

	return nil
}

func (s *session) join(ch ssh.Channel, scx *ServerContext, mode types.SessionParticipantMode) error {
	if scx.Identity.TeleportUser != s.initiator {
		accessContext := auth.SessionAccessContext{
			Username: scx.Identity.TeleportUser,
			Roles:    scx.Identity.AccessChecker.Roles(),
		}

		modes := s.access.CanJoin(accessContext)
		if !slices.Contains(modes, mode) {
			return trace.AccessDenied("insufficient permissions to join session %v", s.id)
		}

		if s.presenceEnabled {
			_, _, err := scx.ServerConn.SendRequest(teleport.MFAPresenceRequest, false, nil)
			if err != nil {
				return trace.WrapWithMessage(err, "failed to send MFA presence request")
			}
		}
	}

	// create a new "party" (connected client) and launch/join the session.
	p := newParty(s, mode, ch, scx)
	if err := s.addParty(p, mode); err != nil {
		return trace.Wrap(err)
	}

	s.logger.DebugContext(s.serverCtx, "Tracking participant.", "party", p)
	participant := &types.Participant{
		ID:         p.id.String(),
		User:       p.user,
		Mode:       string(p.mode),
		LastActive: time.Now().UTC(),
	}
	if err := s.tracker.AddParticipant(s.serverCtx, participant); err != nil {
		return trace.Wrap(err)
	}

	// Emit session join event to both the Audit Log as well as over the
	// "x-teleport-event" channel in the SSH connection.
	s.emitSessionJoinEvent(p.ctx)

	return nil
}

func (s *session) getParties() (parties []*party) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.parties {
		parties = append(parties, p)
	}
	return parties
}

type party struct {
	sync.Mutex

	log        *log.Entry
	login      string
	user       string
	serverID   string
	site       string
	id         rsession.ID
	s          *session
	sconn      *ssh.ServerConn
	ch         ssh.Channel
	ctx        *ServerContext
	lastActive time.Time
	mode       types.SessionParticipantMode
	closeOnce  sync.Once
}

func newParty(s *session, mode types.SessionParticipantMode, ch ssh.Channel, ctx *ServerContext) *party {
	return &party{
		log: log.WithFields(log.Fields{
			teleport.ComponentKey: teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
		}),
		user:     ctx.Identity.TeleportUser,
		login:    ctx.Identity.Login,
		serverID: s.registry.Srv.ID(),
		site:     ctx.ServerConn.RemoteAddr().String(),
		id:       rsession.NewID(),
		ch:       ch,
		ctx:      ctx,
		s:        s,
		sconn:    ctx.ServerConn,
		mode:     mode,
	}
}

func (p *party) updateActivity() {
	p.Lock()
	defer p.Unlock()
	p.lastActive = time.Now()
}

func (p *party) Read(bytes []byte) (int, error) {
	p.updateActivity()
	return p.ch.Read(bytes)
}

func (p *party) Write(bytes []byte) (int, error) {
	return p.ch.Write(bytes)
}

func (p *party) String() string {
	return fmt.Sprintf("%v party(id=%v)", p.ctx, p.id)
}

func (p *party) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", p.id.String()),
		slog.String("remote_addr", p.sconn.RemoteAddr().String()),
	)
}

// Close is called when the party's session ctx is closed.
func (p *party) Close() error {
	p.s.mu.Lock()
	defer p.s.mu.Unlock()
	return p.closeUnderSessionLock()
}

// closeUnderSessionLock closes the party, and removes it from its session.
// Must be called under session Lock.
func (p *party) closeUnderSessionLock() error {
	var err error
	p.closeOnce.Do(func() {
		p.log.Infof("Closing party %v", p.id)
		// Remove party from its session
		err = trace.NewAggregate(p.s.removePartyUnderLock(p), p.ch.Close())
	})

	return err
}

// trackSession creates a new session tracker for the ssh session.
// While ctx is open, the session tracker's expiration will be extended
// on an interval until the session tracker is closed.
func (s *session) trackSession(ctx context.Context, teleportUser string, policySet []*types.SessionTrackerPolicySet, p *party, sessType sessionType) error {
	s.logger.DebugContext(ctx, "Tracking participant.", "party", p)
	var initialCommand []string
	if execRequest, err := s.scx.GetExecRequest(); err == nil {
		initialCommand = []string{execRequest.GetCommand()}
	}
	trackerSpec := types.SessionTrackerSpecV1{
		SessionID:    s.id.String(),
		Kind:         string(types.SSHSessionKind),
		State:        types.SessionState_SessionStatePending,
		Hostname:     s.serverMeta.ServerHostname,
		Address:      s.serverMeta.ServerID,
		ClusterName:  s.scx.ClusterName,
		Login:        s.login,
		HostUser:     teleportUser,
		Reason:       s.scx.env[teleport.EnvSSHSessionReason],
		HostPolicies: policySet,
		Created:      s.registry.clock.Now().UTC(),
		Participants: []types.Participant{
			{
				ID:         p.id.String(),
				User:       p.user,
				Mode:       string(p.mode),
				LastActive: s.registry.clock.Now().UTC(),
			},
		},
		HostID:         s.registry.Srv.ID(),
		TargetSubKind:  s.serverMeta.ServerSubKind,
		InitialCommand: initialCommand,
	}

	if invitedUsers := s.scx.env[teleport.EnvSSHSessionInvited]; invitedUsers != "" {
		if err := json.Unmarshal([]byte(invitedUsers), &trackerSpec.Invited); err != nil {
			return trace.Wrap(err)
		}
	}

	svc := s.registry.SessionTrackerService
	// Only propagate the session tracker when the recording mode and component are in sync
	// AND the sesssion is interactive
	// AND the session was not initiated by a bot
	if (s.registry.Srv.Component() == teleport.ComponentNode && services.IsRecordAtProxy(s.scx.SessionRecordingConfig.GetMode())) ||
		(s.registry.Srv.Component() == teleport.ComponentProxy && !services.IsRecordAtProxy(s.scx.SessionRecordingConfig.GetMode())) ||
		sessType == sessionTypeNonInteractive ||
		s.scx.Identity.BotName != "" {
		svc = nil
	}

	s.logger.DebugContext(ctx, "Attempting to create session tracker.")
	tracker, err := NewSessionTracker(ctx, trackerSpec, svc)
	switch {
	// there was an error creating the tracker for a moderated session - terminate the session
	case err != nil && svc != nil && s.access.IsModerated():
		s.logger.WarnContext(ctx, "Failed to create session tracker, unable to proceed for moderated session", "error", err)
		return trace.Wrap(err)
	// there was an error creating the tracker for a non-moderated session - permit the session with a local tracker
	case err != nil && svc != nil && !s.access.IsModerated():
		s.logger.WarnContext(ctx, "Failed to create session tracker, proceeding with local session tracker for non-moderated session", "error", err)

		localTracker, err := NewSessionTracker(ctx, trackerSpec, nil)
		// this error means there are problems with the trackerSpec, we need to return it
		if err != nil {
			return trace.Wrap(err)
		}

		s.tracker = localTracker
	// there was an error even though the tracker wasn't being propagated - return it
	case err != nil && svc == nil:
		return trace.Wrap(err)
	// the tracker was created successfully
	case err == nil:
		s.tracker = tracker
	}

	go func() {
		ctx, span := tracing.DefaultProvider().Tracer("session").Start(
			s.serverCtx,
			"session/UpdateExpirationLoop",
			oteltrace.WithLinks(oteltrace.LinkFromContext(ctx)),
			oteltrace.WithAttributes(
				attribute.String("session_id", s.id.String()),
				attribute.String("kind", string(types.SSHSessionKind)),
			),
		)
		defer span.End()

		if err := s.tracker.UpdateExpirationLoop(ctx, s.registry.clock); err != nil {
			s.logger.WarnContext(ctx, "Failed to update session tracker expiration.", "error", err)
		}
	}()

	return nil
}

// emitAuditEvent emits audit events.
func (s *session) emitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return s.emitter.EmitAuditEvent(ctx, event)
}

func (s *session) recordEvent(ctx context.Context, event apievents.PreparedSessionEvent) error {
	rec := s.Recorder()
	select {
	case <-rec.Done():
		s.setRecorder(events.WithNoOpPreparer(events.NewDiscardRecorder()))
		return nil
	default:
		return trace.Wrap(rec.RecordEvent(ctx, event))
	}
}

// onWriteError defines the `OnWriteError` `TermManager` callback.
func (s *session) onWriteError(idString string, err error) {
	if idString == sessionRecorderID {
		switch s.scx.Identity.AccessChecker.SessionRecordingMode(constants.SessionRecordingServiceSSH) {
		case constants.SessionRecordingModeBestEffort:
			s.logger.WarnContext(s.serverCtx, "Failed to write to session recorder, disabling session recording.")
			// Send inside a goroutine since the callback is called from inside
			// the writer.
			go s.BroadcastSystemMessage(sessionRecordingWarningMessage)
		default:
			s.logger.ErrorContext(s.serverCtx, "Failed to write to session recorder, stopping session.")
			// stop in goroutine to avoid deadlock
			go func() {
				s.BroadcastSystemMessage(sessionRecordingErrorMessage)
				s.Stop()
			}()
		}
	}
}
