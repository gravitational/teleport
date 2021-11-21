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
	"strings"
	"sync"

	broadcast "github.com/dustin/go-broadcast"
	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
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
	resizeQueue() remotecommand.TerminalSizeQueue
	io.Closer
}

type kubeProxyClientStreams struct {
	proxy     remoteCommandProxy
	sizeQueue remotecommand.TerminalSizeQueue
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
}

func newKubeProxyClientStreams(proxy remoteCommandProxy) *kubeProxyClientStreams {
	options := proxy.options()

	return &kubeProxyClientStreams{
		proxy:  proxy,
		stdin:  options.Stdin,
		stdout: options.Stdout,
		stderr: options.Stderr,
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

func (p *kubeProxyClientStreams) resizeQueue() remotecommand.TerminalSizeQueue {
	return p.sizeQueue
}

func (p *kubeProxyClientStreams) Close() error {
	return trace.Wrap(p.proxy.Close())
}

type party struct {
	Ctx       authContext
	Id        uuid.UUID
	Client    remoteClient
	closeOnce sync.Once
}

func newParty(ctx authContext, client remoteClient) *party {
	return &party{
		Ctx:    ctx,
		Id:     uuid.New(),
		Client: client,
	}
}

func (p *party) Close() error {
	var err error

	p.closeOnce.Do(func() {
		err = p.Client.Close()
	})

	return trace.Wrap(err)
}

// TODO(joel): handle transition to pending on leave
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

	clients_stdin *utils.TrackingReader

	clients_stdout *utils.TrackingWriter

	clients_stderr *utils.TrackingWriter

	terminalSizeQueue remotecommand.TerminalSizeQueue

	state types.SessionState

	stateUpdate broadcast.Broadcaster

	accessEvaluator auth.SessionAccessEvaluator

	recorder events.StreamWriter

	emitter apievents.Emitter

	closeC chan struct{}

	closeOnce sync.Once
}

func newSession(ctx authContext, forwarder *Forwarder, req *http.Request, params httprouter.Params) *session {
	id := uuid.New()
	// TODO(joel): supply roles
	accessEvaluator := auth.NewSessionAccessEvaluator(nil, types.KubernetesSessionKind)

	return &session{
		ctx:               ctx,
		forwarder:         forwarder,
		req:               req,
		params:            params,
		id:                id,
		parties:           make(map[uuid.UUID]*party),
		partiesHistorical: make(map[uuid.UUID]*party),
		log:               forwarder.log.WithField("session", id.String()),
		clients_stdin:     utils.NewTrackingReader(kubeutils.NewMultiReader()),
		clients_stdout:    utils.NewTrackingWriter(srv.NewMultiWriter()),
		clients_stderr:    utils.NewTrackingWriter(srv.NewMultiWriter()),
		state:             types.SessionState_SessionStatePending,
		stateUpdate:       broadcast.NewBroadcaster(1),
		accessEvaluator:   accessEvaluator,
		emitter:           events.NewDiscardEmitter(),
	}
}

// TODO(joel): resize events

func (s *session) launch() error {
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

	sess, err := s.forwarder.newClusterSession(s.ctx)
	sessionStart := s.forwarder.cfg.Clock.Now().UTC()
	if err != nil {
		s.log.Errorf("Failed to create cluster session: %v.", err)
		return trace.Wrap(err)
	}

	eventPodMeta := request.eventPodMeta(request.context, sess.creds)

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
		// TODO(joel); send error to client proxy here
		//if err := s.proxy.sendStatus(err); err != nil {
		//	f.log.WithError(err).Warning("Failed to send status. Exec command was aborted by client.")
		//}

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
				BytesTransmitted: s.clients_stdin.Count(),
				// Bytes received from pod by user.
				BytesReceived: s.clients_stdout.Count() + s.clients_stderr.Count(),
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
				Interactive: true,
				// TODO(joel): participant list here
				Participants:              []string{s.ctx.User.GetName()},
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

	if err = executor.Stream(options); err != nil {
		s.log.WithError(err).Warning("Executor failed while streaming.")
		return trace.Wrap(err)
	}

	return nil
}

// TODO(joel): handle noninteractive sessions
func (s *session) join(p *party) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stringId := p.Id.String()
	s.parties[p.Id] = p
	s.partiesHistorical[p.Id] = p
	s.clients_stdin.R.(*kubeutils.MultiReader).AddReader(stringId, p.Client.stdinStream())

	stdout := kubeutils.WriterCloserWrapper{Writer: p.Client.stdoutStream()}
	s.clients_stdout.W.(*srv.MultiWriter).AddWriter(stringId, stdout, false)

	stderr := kubeutils.WriterCloserWrapper{Writer: p.Client.stderrStream()}
	s.clients_stderr.W.(*srv.MultiWriter).AddWriter(stringId, stderr, false)

	canStart, err := s.canStart()
	if err != nil {
		return trace.Wrap(err)
	}

	if canStart {
		go func() {
			if err := s.launch(); err != nil {
				s.log.WithError(err).Warning("Failed to launch Kubernetes session.")
			}
		}()
	}

	return nil
}

func (s *session) leave(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stringId := id.String()
	party := s.parties[id]
	delete(s.parties, id)
	s.clients_stdin.R.(*kubeutils.MultiReader).RemoveReader(stringId)
	s.clients_stdout.W.(*srv.MultiWriter).DeleteWriter(stringId)
	s.clients_stderr.W.(*srv.MultiWriter).DeleteWriter(stringId)

	err := party.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	if len(s.parties) == 0 {
		err := s.Close()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *session) canStart() (bool, error) {
	// TODO(joel): supply participants
	yes, err := s.accessEvaluator.FulfilledFor(nil)
	return yes, trace.Wrap(err)
}

// TODO(joel): disconnect parties
func (s *session) Close() error {
	s.closeOnce.Do(func() {
		s.log.Infof("Closing session %v.", s.id.String)
		if s.recorder != nil {
			s.recorder.Close(context.TODO())
		}
		close(s.closeC)
	})

	return nil
}
