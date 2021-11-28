/*
Copyright 2021 Gravitational, Inc.

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

package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	broadcast "github.com/dustin/go-broadcast"
	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	tsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

type remoteClient interface {
	stdinStream() io.Reader
	stdoutStream() io.Writer
	stderrStream() io.Writer
	resizeQueue() chan *remotecommand.TerminalSize
	sendStatus(error) error
	waitOnCloseRequest()
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

func (p *websocketClientStreams) resizeQueue() chan *remotecommand.TerminalSize {
	return p.stream.ResizeQueue()
}

func (p *websocketClientStreams) sendStatus(err error) error {
	return nil
}

func (p *websocketClientStreams) waitOnCloseRequest() {
	p.stream.WaitOnClose()
}

func (p *websocketClientStreams) Close() error {
	return trace.Wrap(p.stream.Close())
}

type kubeProxyClientStreams struct {
	proxy     *remoteCommandProxy
	sizeQueue remotecommand.TerminalSizeQueue
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	close     chan struct{}
}

func newKubeProxyClientStreams(proxy *remoteCommandProxy) *kubeProxyClientStreams {
	options := proxy.options()

	return &kubeProxyClientStreams{
		proxy:  proxy,
		stdin:  options.Stdin,
		stdout: options.Stdout,
		stderr: options.Stderr,
		close:  make(chan struct{}),
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

func (p *kubeProxyClientStreams) resizeQueue() chan *remotecommand.TerminalSize {
	ch := make(chan *remotecommand.TerminalSize)
	go func() {
		for {
			size := p.sizeQueue.Next()
			if size == nil {
				break
			}

			ch <- size
		}
	}()

	return ch
}

func (p *kubeProxyClientStreams) waitOnCloseRequest() {
	<-p.close
}

func (p *kubeProxyClientStreams) sendStatus(err error) error {
	return trace.Wrap(p.proxy.sendStatus(err))
}

func (p *kubeProxyClientStreams) Close() error {
	close(p.close)
	return trace.Wrap(p.proxy.Close())
}

type multiResizeQueue struct {
	queues   map[string]chan *remotecommand.TerminalSize
	cases    []reflect.SelectCase
	callback func(*remotecommand.TerminalSize)
}

func (r *multiResizeQueue) rebuild() {
	r.cases = nil
	for _, queue := range r.queues {
		r.cases = append(r.cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(queue),
		})
	}
}

func (r *multiResizeQueue) add(id string, queue chan *remotecommand.TerminalSize) {
	r.queues[id] = queue
	r.rebuild()
}

func (r *multiResizeQueue) remove(id string) {
	delete(r.queues, id)
	r.rebuild()
}

func (r *multiResizeQueue) Next() *remotecommand.TerminalSize {
	_, value, ok := reflect.Select(r.cases)
	if !ok {
		return nil
	}

	size := value.Interface().(*remotecommand.TerminalSize)
	r.callback(size)
	return size
}

type party struct {
	Ctx       authContext
	Id        uuid.UUID
	Client    remoteClient
	closeC    chan struct{}
	closeOnce sync.Once
}

func newParty(ctx authContext, client remoteClient) *party {
	return &party{
		Ctx:    ctx,
		Id:     uuid.New(),
		Client: client,
		closeC: make(chan struct{}),
	}
}

func (p *party) Close() error {
	var err error

	p.closeOnce.Do(func() {
		close(p.closeC)
		err = p.Client.Close()
	})

	return trace.Wrap(err)
}

type session struct {
	mu sync.RWMutex

	ctx authContext

	forwarder *Forwarder

	req *http.Request

	params httprouter.Params

	id uuid.UUID

	parties map[uuid.UUID]*party

	partiesHistorical map[uuid.UUID]*party

	log *log.Entry

	clients_stdin *kubeutils.BreakReader

	clients_stdout *kubeutils.SwitchWriter

	clients_stderr *kubeutils.SwitchWriter

	terminalSizeQueue *multiResizeQueue

	state types.SessionState

	stateUpdate broadcast.Broadcaster

	accessEvaluator auth.SessionAccessEvaluator

	recorder events.StreamWriter

	emitter apievents.Emitter

	tty bool

	podName string

	started bool

	closeC chan struct{}

	closeOnce sync.Once
}

func newSession(ctx authContext, forwarder *Forwarder, req *http.Request, params httprouter.Params, initiator *party) (*session, error) {
	id := uuid.New()
	roles, err := getRolesByName(forwarder, ctx.Context.Identity.GetIdentity().Groups)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessEvaluator := auth.NewSessionAccessEvaluator(roles, types.KubernetesSessionKind)

	s := &session{
		ctx:               ctx,
		forwarder:         forwarder,
		req:               req,
		params:            params,
		id:                id,
		parties:           make(map[uuid.UUID]*party),
		partiesHistorical: make(map[uuid.UUID]*party),
		log:               forwarder.log.WithField("session", id.String()),
		clients_stdin:     kubeutils.NewBreakReader(utils.NewTrackingReader(kubeutils.NewMultiReader())),
		clients_stdout:    kubeutils.NewSwitchWriter(utils.NewTrackingWriter(srv.NewMultiWriter())),
		clients_stderr:    kubeutils.NewSwitchWriter(utils.NewTrackingWriter(srv.NewMultiWriter())),
		state:             types.SessionState_SessionStatePending,
		stateUpdate:       broadcast.NewBroadcaster(1),
		accessEvaluator:   accessEvaluator,
		emitter:           events.NewDiscardEmitter(),
		terminalSizeQueue: &multiResizeQueue{},
		tty:               false,
	}

	err = s.trackerCreate(initiator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}

func (s *session) waitOnAccess() {
	s.clients_stdin.Off()
	s.clients_stdout.Off()
	s.clients_stderr.Off()

	c := make(chan interface{})
	s.stateUpdate.Register(c)
	defer s.stateUpdate.Unregister(c)

outer:
	for {
		state := <-c
		actualState := state.(types.SessionState)
		switch actualState {
		case types.SessionState_SessionStatePending:
			continue
		case types.SessionState_SessionStateTerminated:
			return
		case types.SessionState_SessionStateRunning:
			break outer
		}
	}

	s.clients_stdin.On()
	s.clients_stdout.On()
	s.clients_stderr.On()
}

func (s *session) launch() error {
	s.mu.Lock()

	q := s.req.URL.Query()
	request := remoteCommandRequest{
		podNamespace:       s.params.ByName("podNamespace"),
		podName:            s.params.ByName("podName"),
		containerName:      q.Get("container"),
		cmd:                q["command"],
		stdin:              utils.AsBool(q.Get("stdin")),
		stdout:             utils.AsBool(q.Get("stdout")),
		stderr:             utils.AsBool(q.Get("stderr")),
		tty:                utils.AsBool(q.Get("tty")),
		httpRequest:        s.req,
		httpResponseWriter: nil,
		context:            s.req.Context(),
		pingPeriod:         s.forwarder.cfg.ConnPingPeriod,
	}

	err := s.trackerUpdateState(types.SessionState_SessionStateRunning)
	if err != nil {
		return trace.Wrap(err)
	}

	s.started = true
	s.tty = request.tty
	sess, err := s.forwarder.newClusterSession(s.ctx)
	sessionStart := s.forwarder.cfg.Clock.Now().UTC()
	if err != nil {
		s.log.Errorf("Failed to create cluster session: %v.", err)
		return trace.Wrap(err)
	}

	eventPodMeta := request.eventPodMeta(request.context, sess.creds)

	onWriterError := func(idString string) {
		s.log.Errorf("Encountered error with party %v. Disconnecting them from the session.", idString)
		id, _ := uuid.Parse(idString)
		err := s.leave(id)
		if err != nil {
			s.log.Errorf("Failed to disconnect party %v from the session: %v.", idString, err)
		}
	}

	s.clients_stdout.W.W.(*srv.MultiWriter).OnError = onWriterError
	s.clients_stderr.W.W.(*srv.MultiWriter).OnError = onWriterError

	if s.tty {
		s.terminalSizeQueue.callback = func(resize *remotecommand.TerminalSize) {
			params := tsession.TerminalParams{
				W: int(resize.Width),
				H: int(resize.Height),
			}

			resizeEvent := &apievents.Resize{
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
				SessionMetadata: apievents.SessionMetadata{
					SessionID: s.id.String(),
					WithMFA:   s.ctx.Identity.GetIdentity().MFAVerified,
				},
				UserMetadata: apievents.UserMetadata{
					User:         s.ctx.User.GetName(),
					Login:        s.ctx.User.GetName(),
					Impersonator: s.ctx.Identity.GetIdentity().Impersonator,
				},
				TerminalSize:              params.Serialize(),
				KubernetesClusterMetadata: s.ctx.eventClusterMeta(),
				KubernetesPodMetadata:     eventPodMeta,
			}

			// Report the updated window size to the event log (this is so the sessions
			// can be replayed correctly).
			if err := s.recorder.EmitAuditEvent(s.forwarder.ctx, resizeEvent); err != nil {
				s.forwarder.log.WithError(err).Warn("Failed to emit terminal resize event.")
			}
		}
	} else {
		s.terminalSizeQueue.callback = func(resize *remotecommand.TerminalSize) {}
	}

	if !sess.noAuditEvents && request.tty {
		streamer, err := s.forwarder.newStreamer(&s.ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		recorder, err := events.NewAuditWriter(events.AuditWriterConfig{
			// Audit stream is using server context, not session context,
			// to make sure that session is uploaded even after it is closed
			Context:      s.forwarder.ctx,
			Streamer:     streamer,
			Clock:        s.forwarder.cfg.Clock,
			SessionID:    tsession.ID(s.id.String()),
			ServerID:     s.forwarder.cfg.ServerID,
			Namespace:    s.forwarder.cfg.Namespace,
			RecordOutput: s.ctx.recordingConfig.GetMode() != types.RecordOff,
			Component:    teleport.Component(teleport.ComponentSession, teleport.ComponentProxyKube),
			ClusterName:  s.forwarder.cfg.ClusterName,
		})

		s.recorder = recorder
		s.emitter = recorder
		defer recorder.Close(s.forwarder.ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		s.clients_stdout.W.W.(*srv.MultiWriter).AddWriter("recorder", kubeutils.WriterCloserWrapper{Writer: recorder}, false)
		s.clients_stderr.W.W.(*srv.MultiWriter).AddWriter("recorder", kubeutils.WriterCloserWrapper{Writer: recorder}, false)
	} else if !sess.noAuditEvents {
		s.emitter = s.forwarder.cfg.StreamEmitter
	}

	if request.tty {
		termParams := tsession.TerminalParams{
			W: 100,
			H: 100,
		}

		sessionStartEvent := &apievents.SessionStart{
			Metadata: apievents.Metadata{
				Type:        events.SessionStartEvent,
				Code:        events.SessionStartCode,
				ClusterName: s.forwarder.cfg.ClusterName,
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerID:        s.forwarder.cfg.ServerID,
				ServerNamespace: s.forwarder.cfg.Namespace,
				ServerHostname:  sess.teleportCluster.name,
				ServerAddr:      sess.kubeAddress,
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: s.id.String(),
				WithMFA:   s.ctx.Identity.GetIdentity().MFAVerified,
			},
			UserMetadata: apievents.UserMetadata{
				User:         s.ctx.User.GetName(),
				Login:        s.ctx.User.GetName(),
				Impersonator: s.ctx.Identity.GetIdentity().Impersonator,
			},
			ConnectionMetadata: apievents.ConnectionMetadata{
				RemoteAddr: s.req.RemoteAddr,
				LocalAddr:  sess.kubeAddress,
				Protocol:   events.EventProtocolKube,
			},
			TerminalSize:              termParams.Serialize(),
			KubernetesClusterMetadata: s.ctx.eventClusterMeta(),
			KubernetesPodMetadata:     eventPodMeta,
			InitialCommand:            q["command"],
			SessionRecording:          s.ctx.recordingConfig.GetMode(),
		}

		if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, sessionStartEvent); err != nil {
			s.forwarder.log.WithError(err).Warn("Failed to emit event.")
		}
	}

	if err := s.forwarder.setupForwardingHeaders(sess, s.req); err != nil {
		return trace.Wrap(err)
	}

	executor, err := s.forwarder.getExecutor(s.ctx, sess, s.req)
	if err != nil {
		s.log.WithError(err).Warning("Failed creating executor.")
		return trace.Wrap(err)
	}

	defer func() {
		for _, party := range s.parties {
			if err := party.Client.sendStatus(err); err != nil {
				s.forwarder.log.WithError(err).Warning("Failed to send status. Exec command was aborted by client.")
			}
		}

		if request.tty {
			sessionDataEvent := &apievents.SessionData{
				Metadata: apievents.Metadata{
					Type:        events.SessionDataEvent,
					Code:        events.SessionDataCode,
					ClusterName: s.forwarder.cfg.ClusterName,
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        s.forwarder.cfg.ServerID,
					ServerNamespace: s.forwarder.cfg.Namespace,
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: s.id.String(),
					WithMFA:   s.ctx.Identity.GetIdentity().MFAVerified,
				},
				UserMetadata: apievents.UserMetadata{
					User:         s.ctx.User.GetName(),
					Login:        s.ctx.User.GetName(),
					Impersonator: s.ctx.Identity.GetIdentity().Impersonator,
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					RemoteAddr: s.req.RemoteAddr,
					LocalAddr:  sess.kubeAddress,
					Protocol:   events.EventProtocolKube,
				},
				// Bytes transmitted from user to pod.
				BytesTransmitted: s.clients_stdin.R.Count(),
				// Bytes received from pod by user.
				BytesReceived: s.clients_stdout.W.Count() + s.clients_stderr.W.Count(),
			}

			if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, sessionDataEvent); err != nil {
				s.forwarder.log.WithError(err).Warn("Failed to emit session data event.")
			}

			sessionEndEvent := &apievents.SessionEnd{
				Metadata: apievents.Metadata{
					Type:        events.SessionEndEvent,
					Code:        events.SessionEndCode,
					ClusterName: s.forwarder.cfg.ClusterName,
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        s.forwarder.cfg.ServerID,
					ServerNamespace: s.forwarder.cfg.Namespace,
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: s.id.String(),
					WithMFA:   s.ctx.Identity.GetIdentity().MFAVerified,
				},
				UserMetadata: apievents.UserMetadata{
					User:         s.ctx.User.GetName(),
					Login:        s.ctx.User.GetName(),
					Impersonator: s.ctx.Identity.GetIdentity().Impersonator,
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					RemoteAddr: s.req.RemoteAddr,
					LocalAddr:  sess.kubeAddress,
					Protocol:   events.EventProtocolKube,
				},
				Interactive:               true,
				Participants:              s.allParticipants(),
				StartTime:                 sessionStart,
				EndTime:                   s.forwarder.cfg.Clock.Now().UTC(),
				KubernetesClusterMetadata: s.ctx.eventClusterMeta(),
				KubernetesPodMetadata:     eventPodMeta,
				InitialCommand:            request.cmd,
				SessionRecording:          s.ctx.recordingConfig.GetMode(),
			}

			if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, sessionEndEvent); err != nil {
				s.forwarder.log.WithError(err).Warn("Failed to emit session end event.")
			}
		} else {
			// send an exec event
			execEvent := &apievents.Exec{
				Metadata: apievents.Metadata{
					Type:        events.ExecEvent,
					ClusterName: s.forwarder.cfg.ClusterName,
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        s.forwarder.cfg.ServerID,
					ServerNamespace: s.forwarder.cfg.Namespace,
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: s.id.String(),
					WithMFA:   s.ctx.Identity.GetIdentity().MFAVerified,
				},
				UserMetadata: apievents.UserMetadata{
					User:         s.ctx.User.GetName(),
					Login:        s.ctx.User.GetName(),
					Impersonator: s.ctx.Identity.GetIdentity().Impersonator,
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					RemoteAddr: s.req.RemoteAddr,
					LocalAddr:  sess.kubeAddress,
					Protocol:   events.EventProtocolKube,
				},
				CommandMetadata: apievents.CommandMetadata{
					Command: strings.Join(request.cmd, " "),
				},
				KubernetesClusterMetadata: s.ctx.eventClusterMeta(),
				KubernetesPodMetadata:     eventPodMeta,
			}

			if err != nil {
				execEvent.Code = events.ExecFailureCode
				execEvent.Error = err.Error()
				if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
					execEvent.ExitCode = fmt.Sprintf("%d", exitErr.ExitStatus())
				}
			} else {
				execEvent.Code = events.ExecCode
			}

			if err := s.emitter.EmitAuditEvent(s.forwarder.ctx, execEvent); err != nil {
				s.forwarder.log.WithError(err).Warn("Failed to emit event.")
			}
		}
	}()

	options := remotecommand.StreamOptions{
		Stdin:             s.clients_stdin,
		Stdout:            s.clients_stdout,
		Stderr:            s.clients_stderr,
		Tty:               request.tty,
		TerminalSizeQueue: s.terminalSizeQueue,
	}

	s.mu.Unlock()
	if err = executor.Stream(options); err != nil {
		s.log.WithError(err).Warning("Executor failed while streaming.")
		return trace.Wrap(err)
	}

	return nil
}

func (s *session) join(p *party) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.Ctx.User.GetName() != s.ctx.User.GetName() {
		roleNames := p.Ctx.Identity.GetIdentity().Groups
		roles, err := getRolesByName(s.forwarder, roleNames)
		if err != nil {
			return trace.Wrap(err)
		}

		accessContext := auth.SessionAccessContext{
			Roles: roles,
		}

		if !s.accessEvaluator.CanJoin(accessContext) {
			return trace.AccessDenied("insufficient permissions to join sessions")
		}
	}

	stringId := p.Id.String()
	s.parties[p.Id] = p
	s.partiesHistorical[p.Id] = p

	err := s.trackerAddParticipant(p)
	if err != nil {
		return trace.Wrap(err)
	}

	if s.state != types.SessionState_SessionStatePending {
		return nil
	}

	canStart, _, err := s.canStart()
	if err != nil {
		return trace.Wrap(err)
	}

	if s.started {
		s.state = types.SessionState_SessionStateRunning
		s.stateUpdate.Submit(types.SessionState_SessionStateRunning)
		return nil
	}

	if canStart {
		go func() {
			if err := s.launch(); err != nil {
				s.log.WithError(err).Warning("Failed to launch Kubernetes session.")
			}
		}()
	} else if !s.tty {
		return trace.AccessDenied("insufficient permissions to launch non-interactive session")
	}

	if s.tty {
		s.terminalSizeQueue.add(stringId, p.Client.resizeQueue())
		s.clients_stdin.R.R.(*kubeutils.MultiReader).AddReader(stringId, p.Client.stdinStream())
	}

	stdout := kubeutils.WriterCloserWrapper{Writer: p.Client.stdoutStream()}
	s.clients_stdout.W.W.(*srv.MultiWriter).AddWriter(stringId, stdout, false)

	stderr := kubeutils.WriterCloserWrapper{Writer: p.Client.stderrStream()}
	s.clients_stderr.W.W.(*srv.MultiWriter).AddWriter(stringId, stderr, false)

	return nil
}

func (s *session) leave(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stringId := id.String()
	party := s.parties[id]
	delete(s.parties, id)
	s.terminalSizeQueue.remove(stringId)
	s.clients_stdin.R.R.(*kubeutils.MultiReader).RemoveReader(stringId)
	s.clients_stdout.W.W.(*srv.MultiWriter).DeleteWriter(stringId)
	s.clients_stderr.W.W.(*srv.MultiWriter).DeleteWriter(stringId)

	err := s.trackerRemoveParticipant(party.Id.String())
	if err != nil {
		return trace.Wrap(err)
	}

	err = party.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	if len(s.parties) == 0 {
		go func() {
			err := s.Close()
			if err != nil {
				s.log.Errorf("Failed to close session: %v.", err)
			}
		}()
	}

	canStart, options, err := s.canStart()
	if err != nil {
		return trace.Wrap(err)
	}

	if !canStart {
		if options.TerminateOnLeave {
			go func() {
				err := s.Close()
				if err != nil {
					s.log.Errorf("Failed to close session: %v.", err)
				}
			}()
		} else {
			s.state = types.SessionState_SessionStatePending
			s.stateUpdate.Submit(types.SessionState_SessionStatePending)
			go s.waitOnAccess()
		}
	}

	return nil
}

func (s *session) allParticipants() []string {
	var participants []string
	for _, p := range s.partiesHistorical {
		participants = append(participants, p.Ctx.User.GetName())
	}

	return participants
}

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

		participants = append(participants, auth.SessionAccessContext{Roles: roles})
	}

	yes, options, err := s.accessEvaluator.FulfilledFor(participants)
	return yes, options, trace.Wrap(err)
}

func (s *session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closeOnce.Do(func() {
		s.stateUpdate.Submit(types.SessionState_SessionStateTerminated)
		s.stateUpdate.Close()

		s.log.Infof("Closing session %v.", s.id.String)
		err := s.trackerRemove()
		if err != nil {
			s.log.Error("Failed to remove session tracker resource.")
		}

		close(s.closeC)
		for _, party := range s.parties {
			err := party.Close()
			if err != nil {
				s.log.WithError(err).Error("Failed to disconnect party.")
			}
		}

		if s.recorder != nil {
			s.recorder.Close(context.TODO())
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

func (s *session) trackerCreate(p *party) error {
	initator := &types.Participant{
		ID:         p.Id.String(),
		User:       p.Ctx.User.GetName(),
		LastActive: time.Now().UTC(),
	}

	req := &proto.CreateSessionRequest{
		ID:                s.id.String(),
		Namespace:         defaults.Namespace,
		Type:              string(types.KubernetesSessionKind),
		Hostname:          s.podName,
		ClusterName:       s.ctx.teleportCluster.name,
		Login:             "root",
		Initiator:         initator,
		Expires:           time.Now(),
		KubernetesCluster: s.ctx.kubeCluster,
		HostUser:          initator.User,
	}

	_, err := s.forwarder.cfg.AuthClient.CreateSessionTracker(s.forwarder.ctx, req)
	return trace.Wrap(err)
}

func (s *session) trackerRemove() error {
	err := s.forwarder.cfg.AuthClient.RemoveSessionTracker(s.forwarder.ctx, s.id.String())
	return trace.Wrap(err)
}

func (s *session) trackerAddParticipant(participant *party) error {
	req := &proto.UpdateSessionRequest{
		SessionID: s.id.String(),
		Update: &proto.UpdateSessionRequest_AddParticipant{
			AddParticipant: &proto.AddParticipant{
				Participant: &types.Participant{
					ID:         participant.Id.String(),
					User:       participant.Ctx.User.GetName(),
					LastActive: time.Now().UTC(),
				},
			},
		},
	}

	err := s.forwarder.cfg.AuthClient.UpdateSessionTracker(s.forwarder.ctx, req)
	return trace.Wrap(err)
}

func (s *session) trackerRemoveParticipant(participantID string) error {
	req := &proto.UpdateSessionRequest{
		SessionID: s.id.String(),
		Update: &proto.UpdateSessionRequest_RemoveParticipant{
			RemoveParticipant: &proto.RemoveParticipant{
				ParticipantID: participantID,
			},
		},
	}

	err := s.forwarder.cfg.AuthClient.UpdateSessionTracker(s.forwarder.ctx, req)
	return trace.Wrap(err)
}

func (s *session) trackerUpdateState(state types.SessionState) error {
	req := &proto.UpdateSessionRequest{
		SessionID: s.id.String(),
		Update: &proto.UpdateSessionRequest_UpdateState{
			UpdateState: &proto.UpdateState{
				State: state,
			},
		},
	}

	err := s.forwarder.cfg.AuthClient.UpdateSessionTracker(s.forwarder.ctx, req)
	return trace.Wrap(err)
}
