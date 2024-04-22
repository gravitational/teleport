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

package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	kubeapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/remotecommand"
	watchtools "k8s.io/client-go/tools/watch"

	"github.com/gravitational/teleport"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	tsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

const sessionRecorderID = "session-recorder"

const (
	PresenceVerifyInterval = time.Second * 15
	PresenceMaxDifference  = time.Minute
	sessionMaxLifetime     = time.Hour * 24
)

// remoteClient is either a kubectl or websocket client.
type remoteClient interface {
	stdinStream() io.Reader
	stdoutStream() io.Writer
	stderrStream() io.Writer
	resizeQueue() <-chan *remotecommand.TerminalSize
	resize(size *remotecommand.TerminalSize) error
	forceTerminate() <-chan struct{}
	sendStatus(error) error
	io.Closer
}

type websocketClientStreams struct {
	stream *streamproto.SessionStream
}

func (p *websocketClientStreams) stdinStream() io.Reader {
	return p.stream
}

func (p *websocketClientStreams) stdoutStream() io.Writer {
	return p.stream
}

func (p *websocketClientStreams) stderrStream() io.Writer {
	return p.stream
}

func (p *websocketClientStreams) resizeQueue() <-chan *remotecommand.TerminalSize {
	return p.stream.ResizeQueue()
}

func (p *websocketClientStreams) resize(size *remotecommand.TerminalSize) error {
	return p.stream.Resize(size)
}

func (p *websocketClientStreams) forceTerminate() <-chan struct{} {
	return p.stream.ForceTerminateQueue()
}

func (p *websocketClientStreams) sendStatus(err error) error {
	return nil
}

func (p *websocketClientStreams) Close() error {
	return trace.Wrap(p.stream.Close())
}

type kubeProxyClientStreams struct {
	proxy     *remoteCommandProxy
	sizeQueue *termQueue
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	close     chan struct{}
	wg        sync.WaitGroup
}

func newKubeProxyClientStreams(proxy *remoteCommandProxy) *kubeProxyClientStreams {
	options := proxy.options()

	return &kubeProxyClientStreams{
		proxy:     proxy,
		stdin:     options.Stdin,
		stdout:    options.Stdout,
		stderr:    options.Stderr,
		close:     make(chan struct{}),
		sizeQueue: proxy.resizeQueue,
	}
}

func (p *kubeProxyClientStreams) stdinStream() io.Reader {
	return p.stdin
}

func (p *kubeProxyClientStreams) stdoutStream() io.Writer {
	return p.stdout
}

func (p *kubeProxyClientStreams) stderrStream() io.Writer {
	return p.stderr
}

func (p *kubeProxyClientStreams) resizeQueue() <-chan *remotecommand.TerminalSize {
	ch := make(chan *remotecommand.TerminalSize)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			size := p.sizeQueue.Next()
			if size == nil {
				return
			}
			select {
			case ch <- size:
				// Check if the sizeQueue was already terminated.
			case <-p.sizeQueue.done.Done():
				return
			}
		}
	}()

	return ch
}

func (p *kubeProxyClientStreams) resize(size *remotecommand.TerminalSize) error {
	escape := fmt.Sprintf("\x1b[8;%d;%dt", size.Height, size.Width)
	_, err := p.stdout.Write([]byte(escape))
	return trace.Wrap(err)
}

func (p *kubeProxyClientStreams) forceTerminate() <-chan struct{} {
	return make(chan struct{})
}

func (p *kubeProxyClientStreams) sendStatus(err error) error {
	return trace.Wrap(p.proxy.sendStatus(err))
}

func (p *kubeProxyClientStreams) Close() error {
	if p.sizeQueue != nil {
		p.sizeQueue.Close()
	}
	p.wg.Wait()
	return nil
}

// multiResizeQueue is a merged queue of multiple terminal size queues.
type multiResizeQueue struct {
	queues       map[string]<-chan *remotecommand.TerminalSize
	cases        []reflect.SelectCase
	callback     func(*remotecommand.TerminalSize)
	mutex        sync.Mutex
	parentCtx    context.Context
	reloadCtx    context.Context
	reloadCancel context.CancelFunc
}

func newMultiResizeQueue(parentCtx context.Context) *multiResizeQueue {
	ctx, cancel := context.WithCancel(parentCtx)
	return &multiResizeQueue{
		queues:       make(map[string]<-chan *remotecommand.TerminalSize),
		parentCtx:    parentCtx,
		reloadCtx:    ctx,
		reloadCancel: cancel,
	}
}

func (r *multiResizeQueue) rebuild() {
	oldCancel := r.reloadCancel
	defer oldCancel()

	r.reloadCtx, r.reloadCancel = context.WithCancel(r.parentCtx)
	r.cases = make([]reflect.SelectCase, 1, len(r.queues)+1)
	r.cases[0] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(r.reloadCtx.Done()),
	}
	for _, queue := range r.queues {
		r.cases = append(r.cases,
			reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(queue),
			},
		)
	}
}

func (r *multiResizeQueue) close() {
	r.reloadCancel()
}

func (r *multiResizeQueue) add(id string, queue <-chan *remotecommand.TerminalSize) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.queues[id] = queue
	r.rebuild()
}

func (r *multiResizeQueue) remove(id string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.queues, id)
	r.rebuild()
}

func (r *multiResizeQueue) Next() *remotecommand.TerminalSize {
loop:
	for {
		r.mutex.Lock()
		cases := r.cases
		r.mutex.Unlock()
		idx, value, ok := reflect.Select(cases)
		if !ok || idx == 0 {
			select {
			// if parent context is canceled, the session has ended and we should
			// return early. Otherwise, it means that we rebuilt and in that case we should continue.
			case <-r.parentCtx.Done():
				return nil
			default:
				continue loop
			}
		}

		size := value.Interface().(*remotecommand.TerminalSize)
		r.callback(size)
		return size
	}
}

// party represents one participant of the session and their associated state.
type party struct {
	Ctx       authContext
	ID        uuid.UUID
	Client    remoteClient
	Mode      types.SessionParticipantMode
	closeC    chan error
	closeOnce sync.Once
}

// newParty creates a new party.
func newParty(ctx authContext, mode types.SessionParticipantMode, client remoteClient) *party {
	return &party{
		Ctx:    ctx,
		ID:     uuid.New(),
		Client: client,
		Mode:   mode,
		closeC: make(chan error, 1),
	}
}

// InformClose informs the party that he must leave the session.
func (p *party) InformClose(err error) {
	p.closeOnce.Do(func() {
		p.closeC <- err
		close(p.closeC)
	})
}

// CloseConnection closes the party underlying connection.
func (p *party) CloseConnection() error {
	return trace.Wrap(p.Client.Close())
}

// session represents an ongoing k8s session.
type session struct {
	mu sync.RWMutex

	// ctx is the auth context of the session initiator
	ctx authContext

	forwarder *Forwarder

	req *http.Request

	params httprouter.Params

	id uuid.UUID

	// parties is a map of currently active parties.
	parties map[uuid.UUID]*party

	// partiesHistorical is a map of all current previous parties.
	// This is used for audit trails.
	partiesHistorical map[uuid.UUID]*party

	log *log.Entry

	io *srv.TermManager

	terminalSizeQueue *multiResizeQueue

	tracker *srv.SessionTracker

	accessEvaluator auth.SessionAccessEvaluator

	recorder events.SessionPreparerRecorder

	emitter apievents.Emitter

	podName      string
	podNamespace string
	container    string

	started bool

	initiator uuid.UUID

	expires time.Time

	// sess is the clusterSession used to establish this session.
	sess *clusterSession

	closeC chan struct{}

	closeOnce sync.Once

	// PresenceEnabled is set to true if MFA based presence is required.
	PresenceEnabled bool

	// Set if we should broadcast information about participant requirements to the session.
	displayParticipantRequirements bool

	// invitedUsers is a list of users that were invited to the session.
	invitedUsers []string
	// reason is the reason for the session.
	reason string

	// eventsWaiter is used to wait for events to be emitted and goroutines closed
	// when a session is closed.
	eventsWaiter sync.WaitGroup

	streamContext       context.Context
	streamContextCancel context.CancelFunc
	// partiesWg is a sync.WaitGroup that tracks the number of active parties
	// in this session. It's incremented when a party joins a session and
	// decremented when he leaves - it waits until the session leave events
	// are emitted for every party before returning.
	partiesWg sync.WaitGroup
	// terminationErr is set when the session is terminated.
	terminationErr error
}

// newSession creates a new session in pending mode.
func newSession(ctx authContext, forwarder *Forwarder, req *http.Request, params httprouter.Params, initiator *party, sess *clusterSession) (*session, error) {
	id := uuid.New()
	log := forwarder.log.WithField("session", id.String())
	log.Debug("Creating session")

	var policySets []*types.SessionTrackerPolicySet
	roles := ctx.Checker.Roles()
	for _, role := range roles {
		policySet := role.GetSessionPolicySet()
		policySets = append(policySets, &policySet)
	}

	q := req.URL.Query()
	accessEvaluator := auth.NewSessionAccessEvaluator(policySets, types.KubernetesSessionKind, ctx.User.GetName())

	io := srv.NewTermManager()
	streamContext, streamContextCancel := context.WithCancel(forwarder.ctx)
	namespace := params.ByName("podNamespace")
	podName := params.ByName("podName")
	container := q.Get("container")
	s := &session{
		podName:                        podName,
		podNamespace:                   namespace,
		container:                      container,
		ctx:                            ctx,
		forwarder:                      forwarder,
		req:                            req,
		params:                         params,
		id:                             id,
		parties:                        make(map[uuid.UUID]*party),
		partiesHistorical:              make(map[uuid.UUID]*party),
		log:                            log,
		io:                             io,
		accessEvaluator:                accessEvaluator,
		emitter:                        events.NewDiscardEmitter(),
		terminalSizeQueue:              newMultiResizeQueue(streamContext),
		started:                        false,
		sess:                           sess,
		closeC:                         make(chan struct{}),
		initiator:                      initiator.ID,
		expires:                        time.Now().UTC().Add(sessionMaxLifetime),
		PresenceEnabled:                ctx.Identity.GetIdentity().MFAVerified != "",
		displayParticipantRequirements: utils.AsBool(q.Get(teleport.KubeSessionDisplayParticipantRequirementsQueryParam)),
		invitedUsers:                   strings.Split(q.Get(teleport.KubeSessionInvitedQueryParam), ","),
		reason:                         q.Get(teleport.KubeSessionReasonQueryParam),
		streamContext:                  streamContext,
		streamContextCancel:            streamContextCancel,
		partiesWg:                      sync.WaitGroup{},
	}

	s.io.OnWriteError = s.disconnectPartyOnErr
	s.io.OnReadError = s.disconnectPartyOnErr

	s.BroadcastMessage("Creating session with ID: %v...", id.String())
	s.BroadcastMessage(srv.SessionControlsInfoBroadcast)

	go func() {
		if _, open := <-s.io.TerminateNotifier(); open {
			err := s.Close()
			if err != nil {
				s.log.Errorf("Failed to close session: %v.", err)
			}
		}
	}()

	if err := s.trackSession(initiator, policySets); err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}

// disconnectPartyOnErr is called when any party connection returns an error.
// It is used to properly handle client disconnections.
func (s *session) disconnectPartyOnErr(idString string, err error) {
	if idString == sessionRecorderID {
		s.log.Error("Failed to write to session recorder, closing session.")
		s.Close()
		return
	}

	id, uuidParseErr := uuid.Parse(idString)
	if uuidParseErr != nil {
		s.log.WithError(uuidParseErr).Errorf("Unable to decode %q into a UUID.", idString)
		return
	}

	wasActive, leaveErr := s.leave(id)
	if leaveErr != nil {
		s.log.WithError(leaveErr).Errorf("Failed to disconnect party %v from the session.", idString)
	}
	if wasActive {
		// log the error only if it was the reason for the user disconnection.
		s.log.Errorf("Encountered error: %v with party %v. Disconnecting them from the session.", err, idString)
	}
}

// checkPresence checks the presence timestamp of involved moderators
// and kicks them if they are not active.
func (s *session) checkPresence() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, participant := range s.tracker.GetParticipants() {
		if participant.ID == s.initiator.String() {
			continue
		}

		if participant.Mode == string(types.SessionModeratorMode) && time.Now().UTC().After(participant.LastActive.Add(PresenceMaxDifference)) {
			s.log.Debugf("Participant %v is not active, kicking.", participant.ID)
			id, _ := uuid.Parse(participant.ID)
			_, err := s.unlockedLeave(id)
			if err != nil {
				s.log.WithError(err).Warnf("Failed to kick participant %v for inactivity.", participant.ID)
			}
		}
	}

	return nil
}

// launch waits until the session meets access requirements and then transitions the session
// to a running state.
func (s *session) launch(isEphemeralCont bool) error {
	defer func() {
		err := s.Close()
		if err != nil {
			s.log.WithError(err).Errorf("Failed to close session: %v", s.id)
		}
	}()

	s.log.Debugf("Launching session: %v", s.id)

	q := s.req.URL.Query()
	namespace := s.params.ByName("podNamespace")
	podName := s.params.ByName("podName")
	container := q.Get("container")
	request := &remoteCommandRequest{
		podNamespace:       namespace,
		podName:            podName,
		containerName:      container,
		cmd:                q["command"],
		stdin:              utils.AsBool(q.Get("stdin")),
		stdout:             utils.AsBool(q.Get("stdout")),
		stderr:             utils.AsBool(q.Get("stderr")),
		httpRequest:        s.req,
		httpResponseWriter: nil,
		context:            s.req.Context(),
		pingPeriod:         s.forwarder.cfg.ConnPingPeriod,
	}

	s.podName = request.podName
	s.BroadcastMessage("Connecting to %v over K8S", s.podName)

	eventPodMeta := request.eventPodMeta(request.context, s.sess.kubeAPICreds)

	onFinished, err := s.lockedSetupLaunch(request, q, eventPodMeta)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		// The closure captures the err variable pointer so that the variable can
		// be changed by the code below, but when defer runs, it gets the last value.
		onFinished(err)
	}()

	termParams := tsession.TerminalParams{
		W: 100,
		H: 100,
	}

	sessionStartEvent, err := s.recorder.PrepareSessionEvent(&apievents.SessionStart{
		Metadata: apievents.Metadata{
			Type:        events.SessionStartEvent,
			Code:        events.SessionStartCode,
			ClusterName: s.forwarder.cfg.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        s.forwarder.cfg.HostID,
			ServerNamespace: s.forwarder.cfg.Namespace,
			ServerHostname:  s.sess.teleportCluster.name,
			ServerAddr:      s.sess.kubeAddress,
		},
		SessionMetadata: s.getSessionMetadata(),
		UserMetadata:    s.ctx.eventUserMeta(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: s.req.RemoteAddr,
			LocalAddr:  s.sess.kubeAddress,
			Protocol:   events.EventProtocolKube,
		},
		TerminalSize:              termParams.Serialize(),
		KubernetesClusterMetadata: s.ctx.eventClusterMeta(s.req),
		KubernetesPodMetadata:     eventPodMeta,
		InitialCommand:            q["command"],
		SessionRecording:          s.ctx.recordingConfig.GetMode(),
	})
	if err == nil {
		if err := s.recorder.RecordEvent(s.forwarder.ctx, sessionStartEvent); err != nil {
			s.forwarder.log.WithError(err).Warn("Failed to record session start event.")
		}
		if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, sessionStartEvent.GetAuditEvent()); err != nil {
			s.forwarder.log.WithError(err).Warn("Failed to emit session start event.")
		}
	} else {
		s.forwarder.log.WithError(err).Warn("Failed to set up session start event - event will not be recorded")
	}

	s.eventsWaiter.Add(1)
	go func() {
		defer s.eventsWaiter.Done()
		t := time.NewTimer(time.Until(s.expires))
		defer t.Stop()

		select {
		case <-t.C:
			s.BroadcastMessage("Session expired, closing...")
			err := s.Close()
			if err != nil {
				s.log.WithError(err).Error("Failed to close session")
			}
		case <-s.closeC:
		}
	}()

	if err = s.tracker.UpdateState(s.forwarder.ctx, types.SessionState_SessionStateRunning); err != nil {
		s.log.WithError(err).Warn("Failed to set tracker state to running")
	}

	var executor remotecommand.Executor

	executor, err = s.forwarder.getExecutor(s.sess, s.req)
	if err != nil {
		s.log.WithError(err).Warning("Failed creating executor.")
		return trace.Wrap(err)
	}

	options := remotecommand.StreamOptions{
		Stdin:             s.io,
		Stdout:            s.io,
		Stderr:            s.io,
		Tty:               true,
		TerminalSizeQueue: s.terminalSizeQueue,
	}

	s.io.On()
	if streamErr := executor.StreamWithContext(s.streamContext, options); streamErr != nil {
		onErr := func(err error) {
			s.setTerminationErr(err)
			s.reportErrorToSessionRecorder(err)
			s.log.WithError(err).Warning("Executor failed while streaming.")
		}

		if !isEphemeralCont {
			onErr(streamErr)
			return trace.Wrap(streamErr)
		}

		// If attaching to the container failed, check if the container
		// is terminated. If it is, try to stream the logs. If it's not
		// terminated or can't be found return the original error.
		clientSet, _, err := s.forwarder.impersonatedKubeClient(&s.sess.authContext, s.req.Header)
		if err != nil {
			onErr(err)
			return trace.Wrap(err)
		}
		podClient := clientSet.CoreV1().Pods(namespace)

		pod, err := podClient.Get(s.forwarder.ctx, podName, metav1.GetOptions{})
		if err != nil {
			onErr(err)
			return trace.Wrap(err)
		}
		status := getEphemeralContainerStatusByName(pod, container)
		if status == nil {
			// the container couldn't be found in the pod, return the
			// original command streaming error
			onErr(streamErr)
			return trace.Wrap(streamErr)
		}
		if status.State.Terminated != nil {
			if err := s.retrieveAlreadyStoppedPodLogs(
				podClient,
				namespace,
				podName,
				container,
			); err != nil {
				onErr(err)
				return trace.Wrap(err)
			}

			return nil
		}

		onErr(streamErr)
		return trace.Wrap(streamErr)
	}

	return nil
}

func (s *session) setTerminationErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setTerminationErrUnlocked(err)
}

func (s *session) setTerminationErrUnlocked(err error) {
	if s.terminationErr != nil {
		return
	}
	s.terminationErr = err
}

// reportErrorToSessionRecorder reports the error to the session recorder
// if it is set.
func (s *session) reportErrorToSessionRecorder(err error) {
	if err == nil {
		return
	}
	if s.recorder != nil {
		fmt.Fprintf(s.recorder, "\r\n---\r\nSession exited with error: %v\r\n", err)
	}
}

func (s *session) lockedSetupLaunch(request *remoteCommandRequest, q url.Values, eventPodMeta apievents.KubernetesPodMetadata) (func(error), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	s.started = true
	sessionStart := s.forwarder.cfg.Clock.Now().UTC()

	if !s.sess.noAuditEvents {
		s.terminalSizeQueue.callback = func(resize *remotecommand.TerminalSize) {
			s.mu.Lock()
			defer s.mu.Unlock()

			for id, p := range s.parties {
				err := p.Client.resize(resize)
				if err != nil {
					s.log.WithError(err).Errorf("Failed to resize client: %v", id.String())
				}
			}

			params := tsession.TerminalParams{
				W: int(resize.Width),
				H: int(resize.Height),
			}

			resizeEvent, err := s.recorder.PrepareSessionEvent(&apievents.Resize{
				Metadata: apievents.Metadata{
					Type:        events.ResizeEvent,
					Code:        events.TerminalResizeCode,
					ClusterName: s.forwarder.cfg.ClusterName,
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					RemoteAddr: s.req.RemoteAddr,
					Protocol:   events.EventProtocolKube,
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerNamespace: s.forwarder.cfg.Namespace,
				},
				SessionMetadata:           s.getSessionMetadata(),
				UserMetadata:              s.ctx.eventUserMeta(),
				TerminalSize:              params.Serialize(),
				KubernetesClusterMetadata: s.ctx.eventClusterMeta(s.req),
				KubernetesPodMetadata:     eventPodMeta,
			})
			if err == nil {
				// Report the updated window size to the event log (this is so the sessions
				// can be replayed correctly).
				if err := s.recorder.RecordEvent(s.forwarder.ctx, resizeEvent); err != nil {
					s.forwarder.log.WithError(err).Warn("Failed to emit terminal resize event.")
				}
			} else {
				s.forwarder.log.WithError(err).Warn("Failed to set up terminal resize event - event will not be recorded")
			}
		}
	} else {
		s.terminalSizeQueue.callback = func(resize *remotecommand.TerminalSize) {}
	}

	recorder, err := recorder.New(recorder.Config{
		SessionID:    tsession.ID(s.id.String()),
		ServerID:     s.forwarder.cfg.HostID,
		Namespace:    s.forwarder.cfg.Namespace,
		Clock:        s.forwarder.cfg.Clock,
		ClusterName:  s.forwarder.cfg.ClusterName,
		RecordingCfg: s.ctx.recordingConfig,
		SyncStreamer: s.forwarder.cfg.AuthClient,
		DataDir:      s.forwarder.cfg.DataDir,
		Component:    teleport.Component(teleport.ComponentSession, teleport.ComponentProxyKube),
		// Session stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context: s.forwarder.ctx,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.recorder = recorder
	s.emitter = s.forwarder.cfg.Emitter

	s.io.AddWriter(sessionRecorderID, recorder)

	// If the identity is verified with an MFA device, we enabled MFA-based presence for the session.
	if s.PresenceEnabled {
		s.eventsWaiter.Add(1)
		go func() {
			defer s.eventsWaiter.Done()
			ticker := time.NewTicker(PresenceVerifyInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					err := s.checkPresence()
					if err != nil {
						s.log.WithError(err).Error("Failed to check presence, closing session as a security measure")
						err := s.Close()
						if err != nil {
							s.log.WithError(err).Error("Failed to close session")
						}
					}
				case <-s.closeC:
					return
				}
			}
		}()
	}
	// If we get here, it means we are going to have a session.end event.
	// This increments the waiter so that session.Close() guarantees that once called
	// the events are emitted before closing the emitter/recorder.
	// It might happen when a user disconnects or when a moderator forces an early
	// termination.
	s.eventsWaiter.Add(1)
	// receive the exec error returned from API call to kube cluster
	return func(errExec error) {
		defer s.eventsWaiter.Done()
		s.mu.Lock()
		defer s.mu.Unlock()

		serverMetadata := apievents.ServerMetadata{
			ServerID:        s.forwarder.cfg.HostID,
			ServerNamespace: s.forwarder.cfg.Namespace,
		}

		sessionMetadata := s.getSessionMetadata()

		conMetadata := apievents.ConnectionMetadata{
			RemoteAddr: s.req.RemoteAddr,
			LocalAddr:  s.sess.kubeAddress,
			Protocol:   events.EventProtocolKube,
		}

		execEvent := &apievents.Exec{
			Metadata: apievents.Metadata{
				Type:        events.ExecEvent,
				ClusterName: s.forwarder.cfg.ClusterName,
				// can be changed to ExecFailureCode if errExec is not nil
				Code: events.ExecCode,
			},
			ServerMetadata:     serverMetadata,
			SessionMetadata:    sessionMetadata,
			UserMetadata:       s.sess.eventUserMeta(),
			ConnectionMetadata: conMetadata,
			CommandMetadata: apievents.CommandMetadata{
				Command: strings.Join(request.cmd, " "),
			},
			KubernetesClusterMetadata: s.ctx.eventClusterMeta(s.req),
			KubernetesPodMetadata:     eventPodMeta,
		}

		if errExec != nil {
			execEvent.Code = events.ExecFailureCode
			execEvent.Error, execEvent.ExitCode = exitCode(errExec)
		}

		if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, execEvent); err != nil {
			s.forwarder.log.WithError(err).Warn("Failed to emit exec event.")
		}

		sessionDataEvent := &apievents.SessionData{
			Metadata: apievents.Metadata{
				Type:        events.SessionDataEvent,
				Code:        events.SessionDataCode,
				ClusterName: s.forwarder.cfg.ClusterName,
			},
			ServerMetadata:     serverMetadata,
			SessionMetadata:    sessionMetadata,
			UserMetadata:       s.sess.eventUserMeta(),
			ConnectionMetadata: conMetadata,
			// Bytes transmitted from user to pod.
			BytesTransmitted: s.io.CountRead(),
			// Bytes received from pod by user.
			BytesReceived: s.io.CountWritten(),
		}

		if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, sessionDataEvent); err != nil {
			s.forwarder.log.WithError(err).Warn("Failed to emit session data event.")
		}

		sessionEndEvent, err := s.recorder.PrepareSessionEvent(&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type:        events.SessionEndEvent,
				Code:        events.SessionEndCode,
				ClusterName: s.forwarder.cfg.ClusterName,
			},
			ServerMetadata:            serverMetadata,
			SessionMetadata:           sessionMetadata,
			UserMetadata:              s.sess.eventUserMeta(),
			ConnectionMetadata:        conMetadata,
			Interactive:               true,
			Participants:              s.allParticipants(),
			StartTime:                 sessionStart,
			EndTime:                   s.forwarder.cfg.Clock.Now().UTC(),
			KubernetesClusterMetadata: s.ctx.eventClusterMeta(s.req),
			KubernetesPodMetadata:     eventPodMeta,
			InitialCommand:            request.cmd,
			SessionRecording:          s.ctx.recordingConfig.GetMode(),
		})
		if err == nil {
			if err := s.recorder.RecordEvent(s.forwarder.ctx, sessionEndEvent); err != nil {
				s.forwarder.log.WithError(err).Warn("Failed to record session end event.")
			}
			if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, sessionEndEvent.GetAuditEvent()); err != nil {
				s.forwarder.log.WithError(err).Warn("Failed to emit session end event.")
			}
		} else {
			s.forwarder.log.WithError(err).Warn("Failed to set up session end event - event will not be recorded")
		}
	}, nil
}

// join attempts to connect a party to the session.
func (s *session) join(p *party, emitJoinEvent bool) error {
	if p.Ctx.User.GetName() != s.ctx.User.GetName() {
		roles := p.Ctx.Checker.Roles()

		accessContext := auth.SessionAccessContext{
			Username: p.Ctx.User.GetName(),
			Roles:    roles,
		}

		modes := s.accessEvaluator.CanJoin(accessContext)
		if !slices.Contains(modes, p.Mode) {
			return trace.AccessDenied("insufficient permissions to join session")
		}
	}

	if s.tracker.GetState() == types.SessionState_SessionStateTerminated {
		return trace.AccessDenied("The requested session is not active")
	}

	s.log.Debugf("Tracking participant: %s", p.ID)
	participant := &types.Participant{
		ID:         p.ID.String(),
		User:       p.Ctx.User.GetName(),
		Mode:       string(p.Mode),
		LastActive: time.Now().UTC(),
	}
	if err := s.tracker.AddParticipant(s.forwarder.ctx, participant); err != nil {
		return trace.Wrap(err)
	}

	// we only want to emit the session.join when someone tries to join a session via
	// tsh kube join and not when the original session owner terminal streams are
	// connected to the Kubernetes session.
	if emitJoinEvent {
		s.emitSessionJoinEvent(p)
	}

	recentWrites := s.io.GetRecentHistory()
	if _, err := p.Client.stdoutStream().Write(recentWrites); err != nil {
		s.log.Warnf("Failed to write history to client: %v.", err)
	}

	// increment the party track waitgroup.
	// It is decremented when session.leave() finishes its execution.
	s.partiesWg.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	stringID := p.ID.String()
	s.parties[p.ID] = p
	s.partiesHistorical[p.ID] = p
	s.terminalSizeQueue.add(stringID, p.Client.resizeQueue())

	if p.Mode == types.SessionPeerMode {
		s.io.AddReader(stringID, p.Client.stdinStream())
	}

	s.io.AddWriter(stringID, p.Client.stdoutStream())
	s.BroadcastMessage("User %v joined the session with participant mode: %v.", p.Ctx.User.GetName(), p.Mode)

	if p.Mode == types.SessionModeratorMode {
		s.eventsWaiter.Add(1)
		go func() {
			defer s.eventsWaiter.Done()
			c := p.Client.forceTerminate()
			select {
			case <-c:
				s.setTerminationErr(sessionTerminatedByModeratorErr)
				go func() {
					s.log.Debugf("Received force termination request")
					err := s.Close()
					if err != nil {
						s.log.Errorf("Failed to close session: %v.", err)
					}
				}()
			case <-s.closeC:
				return
			}
		}()
	}

	canStart, _, err := s.canStart()
	if err != nil {
		return trace.Wrap(err)
	}

	if !s.started {
		if canStart {
			// create an ephemeral container if this session will be
			// running in one now that the moderated session is approved
			startedEphemeralCont, err := s.createEphemeralContainer()
			if err != nil {
				// if the ephemeral container creation fails, close the session
				// and return the error. We need to close the session here because
				// we must inform all parties that the session is closing.
				s.setTerminationErrUnlocked(err)
				s.reportErrorToSessionRecorder(err)
				s.log.WithError(err).Warning("Executor failed while creating ephemeral pod.")
				go func() {
					err := s.Close()
					if err != nil {
						s.log.WithError(err).Error("Failed to close session")
					}
				}()
				return trace.Wrap(err)
			}

			go func() {
				if err := s.launch(startedEphemeralCont); err != nil {
					s.log.WithError(err).Warning("Failed to launch Kubernetes session.")
				}
			}()
		} else if len(s.parties) == 1 {
			base := "Waiting for required participants..."

			if s.displayParticipantRequirements {
				s.BroadcastMessage(base+"\r\n%v", s.accessEvaluator.PrettyRequirementsList())
			} else {
				s.BroadcastMessage(base)
			}
		}
	} else if canStart && s.tracker.GetState() == types.SessionState_SessionStatePending {
		// If the session is already running, but the party is a moderator that left
		// a session with onLeave=pause and then rejoined, we need to unpause the session.
		// When the moderator left the session, the session was paused, and we spawn
		// a goroutine to wait for the moderator to rejoin. If the moderator rejoins
		// before the session ends, we need to unpause the session by updating its state and
		// the goroutine will unblock the s.io terminal.
		// types.SessionState_SessionStatePending marks a session that is waiting for
		// a moderator to rejoin.
		if err := s.tracker.UpdateState(s.forwarder.ctx, types.SessionState_SessionStateRunning); err != nil {
			s.log.Warnf("Failed to set tracker state to %v", types.SessionState_SessionStateRunning)
		}
	}

	return nil
}

// createEphemeralContainer creates an ephemeral container and waits for it to start.
func (s *session) createEphemeralContainer() (bool, error) {
	initUser := s.parties[s.initiator]
	username := initUser.Ctx.Identity.GetIdentity().Username
	namespace := s.params.ByName("podNamespace")
	podName := s.params.ByName("podName")
	container := s.req.URL.Query().Get("container")

	waitingCont, err := s.forwarder.cfg.CachingAuthClient.GetKubernetesWaitingContainer(
		s.forwarder.ctx,
		&kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest{
			Username:      username,
			Cluster:       s.ctx.kubeClusterName,
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: container,
		},
	)
	if trace.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, trace.Wrap(err)
	}

	if err = s.forwarder.cfg.AuthClient.DeleteKubernetesWaitingContainer(
		s.forwarder.ctx,
		&kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest{
			Username:      username,
			Cluster:       s.ctx.kubeClusterName,
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: container,
		},
	); err != nil {
		return false, trace.Wrap(err)
	}

	s.log.Debugf("Creating ephemeral container %s on pod %s", container, podName)
	err = s.patchAndWaitForPodEphemeralContainer(s.forwarder.ctx, &initUser.Ctx, s.req.Header, waitingCont)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return true, nil
}

func (s *session) BroadcastMessage(format string, args ...any) {
	if s.accessEvaluator.IsModerated() {
		s.io.BroadcastMessage(fmt.Sprintf(format, args...))
	}
}

// emitSessionJoinEvent emits a session.join audit event when a user joins
// the session.
// This function requires that the session must be active, otherwise audit logger
// will discard the event.
func (s *session) emitSessionJoinEvent(p *party) {
	sessionJoinEvent := &apievents.SessionJoin{
		Metadata: apievents.Metadata{
			Type:        events.SessionJoinEvent,
			Code:        events.SessionJoinCode,
			ClusterName: s.ctx.teleportCluster.name,
		},
		KubernetesClusterMetadata: apievents.KubernetesClusterMetadata{
			KubernetesCluster: s.ctx.kubeClusterName,
			// joining moderators, obervers and peers don't have any
			// kubernetes metadata configured.
			KubernetesUsers:  []string{},
			KubernetesGroups: []string{},
			KubernetesLabels: s.ctx.kubeClusterLabels,
		},
		SessionMetadata: s.getSessionMetadata(),
		UserMetadata:    p.Ctx.eventUserMetaWithLogin("root"),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: s.params.ByName("podName"),
		},
	}

	if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, sessionJoinEvent); err != nil {
		s.forwarder.log.WithError(err).Warn("Failed to emit event.")
	}
}

// leave removes a party from the session and returns if the party was still active
// in the session. If the party wasn't found, it returns false, nil.
func (s *session) leave(id uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unlockedLeave(id)
}

// unlockedLeave removes a party from the session without locking the mutex.
// The boolean returned identifies if the party was still active in the session.
// If the party wasn't found, it returns false, nil.
// In order to call this function, lock the mutex before.
func (s *session) unlockedLeave(id uuid.UUID) (bool, error) {
	var errs []error
	stringID := id.String()
	party := s.parties[id]

	if party == nil {
		return false, nil
	}
	// Waits until the function execution ends to release the parties waitgroup.
	// It's used to prevent the session to terminate the events emitter before
	// the session leave event is emitted.
	defer s.partiesWg.Done()
	delete(s.parties, id)
	s.terminalSizeQueue.remove(stringID)
	s.io.DeleteReader(stringID)
	s.io.DeleteWriter(stringID)

	s.BroadcastMessage("User %v left the session.", party.Ctx.User.GetName())

	sessionLeaveEvent := &apievents.SessionLeave{
		Metadata: apievents.Metadata{
			Type:        events.SessionLeaveEvent,
			Code:        events.SessionLeaveCode,
			ClusterName: s.ctx.teleportCluster.name,
		},
		SessionMetadata: s.getSessionMetadata(),
		UserMetadata:    party.Ctx.eventUserMetaWithLogin("root"),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: s.params.ByName("podName"),
		},
	}

	if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, sessionLeaveEvent); err != nil {
		s.forwarder.log.WithError(err).Warn("Failed to emit event.")
	}

	s.log.Debugf("No longer tracking participant: %v", party.ID)
	err := s.tracker.RemoveParticipant(s.forwarder.ctx, party.ID.String())
	if err != nil {
		errs = append(errs, trace.Wrap(err))
	}

	party.InformClose(s.terminationErr)
	if len(s.parties) == 0 || id == s.initiator {
		go func() {
			// Currently, Teleport closes the session when the initiator exits.
			// So, it is safe to remove it
			s.forwarder.deleteSession(s.id)
			// close session
			err := s.Close()
			if err != nil {
				s.log.WithError(err).Errorf("Failed to close session")
			}
		}()
		return true, trace.NewAggregate(errs...)
	}

	// We wait until here to return to check if we should terminate the
	// session.
	if len(errs) > 0 {
		return true, trace.NewAggregate(errs...)
	}

	canStart, options, err := s.canStart()
	if err != nil {
		return true, trace.Wrap(err)
	}

	if !canStart {
		if options.OnLeaveAction == types.OnSessionLeaveTerminate {
			go func() {
				if err := s.Close(); err != nil {
					s.log.WithError(err).Errorf("Failed to close session")
				}
			}()
			return true, nil
		}

		// pause session and wait for another party to resume
		s.io.Off()
		s.BroadcastMessage("Session paused, Waiting for required participants...")
		if err := s.tracker.UpdateState(s.forwarder.ctx, types.SessionState_SessionStatePending); err != nil {
			s.log.Warnf("Failed to set tracker state to %v", types.SessionState_SessionStatePending)
		}

		go func() {
			if state := s.tracker.WaitForStateUpdate(types.SessionState_SessionStatePending); state == types.SessionState_SessionStateRunning {
				s.BroadcastMessage("Resuming session...")
				s.io.On()
			}
		}()
	}

	return true, nil
}

// allParticipants returns a list of all historical participants of the session.
func (s *session) allParticipants() []string {
	var participants []string
	for _, p := range s.partiesHistorical {
		participants = append(participants, p.Ctx.User.GetName())
	}

	return participants
}

// canStart checks if a session can start with the current set of participants.
func (s *session) canStart() (bool, auth.PolicyOptions, error) {
	var participants []auth.SessionAccessContext
	for _, party := range s.parties {
		if party.Ctx.User.GetName() == s.ctx.User.GetName() {
			continue
		}

		roleNames := party.Ctx.Identity.GetIdentity().Groups
		roles, err := getRolesByName(s.forwarder, roleNames)
		if err != nil {
			return false, auth.PolicyOptions{}, trace.Wrap(err)
		}

		participants = append(participants, auth.SessionAccessContext{
			Username: party.Ctx.User.GetName(),
			Roles:    roles,
			Mode:     party.Mode,
		})
	}

	yes, options, err := s.accessEvaluator.FulfilledFor(participants)
	return yes, options, trace.Wrap(err)
}

// Close terminates a session and disconnects all participants.
func (s *session) Close() error {
	s.closeOnce.Do(func() {
		s.BroadcastMessage("Closing session...")

		s.io.Close()
		// Once tracker is closed parties cannot join the session.
		// check session.join for logic.
		if err := s.tracker.Close(s.forwarder.ctx); err != nil {
			s.log.WithError(err).Debug("Failed to close session tracker")
		}
		s.mu.Lock()
		terminationErr := s.terminationErr
		// terminate all active parties in the session.
		for _, party := range s.parties {
			party.InformClose(terminationErr)
		}
		recorder := s.recorder
		s.mu.Unlock()
		s.log.Debugf("Closing session %v.", s.id.String())
		close(s.closeC)
		// Wait until every party leaves the session and emits the session leave
		// event before closing the recorder - if available.
		s.partiesWg.Wait()

		s.streamContextCancel()
		s.terminalSizeQueue.close()
		if recorder != nil {
			// wait for events to be emitted before closing the recorder/emitter.
			// If we close it immediately we will lose session.end events.
			s.eventsWaiter.Wait()
			recorder.Close(s.forwarder.ctx)
		}
	})

	return nil
}

func getRolesByName(forwarder *Forwarder, roleNames []string) ([]types.Role, error) {
	var roles []types.Role

	for _, roleName := range roleNames {
		role, err := forwarder.cfg.CachingAuthClient.GetRole(context.TODO(), roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		roles = append(roles, role)
	}

	return roles, nil
}

// trackSession creates a new session tracker for the kube session.
// While ctx is open, the session tracker's expiration will be extended
// on an interval until the session tracker is closed.
func (s *session) trackSession(p *party, policySet []*types.SessionTrackerPolicySet) error {
	ctx := s.req.Context()
	command := s.req.URL.Query()["command"]
	if len(command) == 0 {
		command = s.retrieveEphemeralContainerCommand(ctx, p.Ctx.User.GetName(), s.req.URL.Query().Get("container"))
	}
	trackerSpec := types.SessionTrackerSpecV1{
		SessionID:         s.id.String(),
		Kind:              string(types.KubernetesSessionKind),
		State:             types.SessionState_SessionStatePending,
		Hostname:          s.podName,
		ClusterName:       s.ctx.teleportCluster.name,
		KubernetesCluster: s.ctx.kubeClusterName,
		HostUser:          p.Ctx.User.GetName(),
		HostPolicies:      policySet,
		Login:             "root",
		Created:           s.forwarder.cfg.Clock.Now(),
		Reason:            s.reason,
		Invited:           s.invitedUsers,
		HostID:            s.forwarder.cfg.HostID,
		InitialCommand:    command,
	}

	s.log.Debug("Creating session tracker")
	sessionTrackerService := s.forwarder.cfg.AuthClient

	tracker, err := srv.NewSessionTracker(ctx, trackerSpec, sessionTrackerService)
	switch {
	// there was an error creating the tracker for a moderated session - terminate the session
	case err != nil && s.accessEvaluator.IsModerated():
		s.log.WithError(err).Warn("Failed to create session tracker, unable to proceed for moderated session")
		return trace.Wrap(err)
	// there was an error creating the tracker for a non-moderated session - permit the session with a local tracker
	case err != nil && !s.accessEvaluator.IsModerated():
		s.log.Warn("Failed to create session tracker, proceeding with local session tracker for non-moderated session")

		localTracker, err := srv.NewSessionTracker(ctx, trackerSpec, nil)
		// this error means there are problems with the trackerSpec, we need to return it
		if err != nil {
			return trace.Wrap(err)
		}

		s.tracker = localTracker
	// there was an error even though the tracker wasn't being propagated - return it
	case err != nil:
		return trace.Wrap(err)
	// the tracker was created successfully
	case err == nil:
		s.tracker = tracker
	}

	go func() {
		if err := s.tracker.UpdateExpirationLoop(s.forwarder.ctx, s.forwarder.cfg.Clock); err != nil {
			s.log.WithError(err).Warn("Failed to update session tracker expiration")
		}
	}()

	return nil
}

func (s *session) getSessionMetadata() apievents.SessionMetadata {
	return s.ctx.Identity.GetIdentity().GetSessionMetadata(s.id.String())
}

// patchPodWithEphemeralContainer creates an ephemeral container and waits
// for it to start.
func (s *session) patchAndWaitForPodEphemeralContainer(ctx context.Context, authCtx *authContext, headers http.Header, waitingCont *kubewaitingcontainerpb.KubernetesWaitingContainer) error {
	fmt.Fprintf(s.io, "\r\nCreating ephemeral container %s in pod %s/%s\r\n", waitingCont.Spec.ContainerName, waitingCont.Spec.Namespace, waitingCont.Spec.PodName)

	clientSet, _, err := s.forwarder.impersonatedKubeClient(authCtx, headers)
	if err != nil {
		return trace.Wrap(err)
	}
	podClient := clientSet.CoreV1().Pods(authCtx.kubeResource.Namespace)
	_, err = podClient.Patch(ctx,
		waitingCont.Spec.PodName,
		kubeapitypes.StrategicMergePatchType,
		waitingCont.Spec.Patch,
		metav1.PatchOptions{},
		"ephemeralcontainers")
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(s.io, "Pod %s/%s successfully patched. Waiting for container to become ready.\r\n",
		waitingCont.Spec.Namespace,
		waitingCont.Spec.PodName)

	fieldSelector := fields.OneTermEqualSelector("metadata.name", waitingCont.Spec.PodName).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return podClient.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return podClient.Watch(ctx, options)
		},
	}
	_, err = watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(ev watch.Event) (bool, error) {
		switch ev.Type {
		case watch.Deleted:
			return false, trace.NotFound("pod %s not found", waitingCont.Spec.PodName)
		}

		p, ok := ev.Object.(*corev1.Pod)
		if !ok {
			return false, trace.BadParameter("watch did not return a pod: %v", ev.Object)
		}

		s := getEphemeralContainerStatusByName(p, waitingCont.Spec.ContainerName)
		if s == nil {
			return false, nil
		}
		if s.State.Running != nil || s.State.Terminated != nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(s.io, "Ephemeral container %s is ready.\r\n", waitingCont.Spec.ContainerName)

	return nil
}

// retrieveAlreadyStoppedPodLogs retrieves the logs of a stopped pod and writes them to the session's io writer.
func (s *session) retrieveAlreadyStoppedPodLogs(podClient clientv1.PodInterface, namespace, podName, container string) error {
	fmt.Fprintf(s.io, "Failed to attach to the container, attempting to stream logs instead...\r\n")
	req := podClient.GetLogs(podName, &corev1.PodLogOptions{Container: container})
	r, err := req.Stream(s.streamContext)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := io.Copy(s.io, r); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.Close())
}

// retrieveEphemeralContainerCommand retrieves the command of an ephemeral container
// if it exists.
func (s *session) retrieveEphemeralContainerCommand(ctx context.Context, username, containerName string) []string {
	containers, err := s.forwarder.getUserEphemeralContainersForPod(ctx, username, s.ctx.kubeClusterName, s.podNamespace, s.podName)
	if err != nil {
		s.log.WithError(err).Warn("Failed to retrieve ephemeral containers")
		return nil
	}
	if len(containers) == 0 {
		return nil
	}
	for _, container := range containers {
		if container.GetMetadata().GetName() != containerName {
			continue
		}

		contentType, err := patchTypeToContentType(apimachinerytypes.PatchType(container.Spec.PatchType))
		if err != nil {
			return nil
		}
		encoder, decoder, err := newEncoderAndDecoderForContentType(
			contentType,
			newClientNegotiator(s.sess.codecFactory),
		)
		if err != nil {
			s.log.WithError(err).Warn("Failed to create encoder and decoder")
			return nil
		}
		pod, _, err := s.forwarder.mergeEphemeralPatchWithCurrentPod(
			ctx,
			mergeEphemeralPatchWithCurrentPodConfig{
				kubeCluster:   s.ctx.kubeClusterName,
				kubeNamespace: s.podNamespace,
				podName:       s.podName,
				decoder:       decoder,
				encoder:       encoder,
				podPatch:      container.GetSpec().Patch,
				patchType:     apimachinerytypes.PatchType(container.GetSpec().PatchType),
			},
		)
		if err != nil {
			s.log.WithError(err).Warn("Failed to merge ephemeral patch with current pod")
			return nil
		}
		for _, ephemeral := range pod.Spec.EphemeralContainers {
			if ephemeral.Name == containerName {
				return ephemeral.Command
			}
		}

	}
	return nil
}
