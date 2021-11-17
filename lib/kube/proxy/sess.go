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
	"io"
	"net/http"
	"sync"

	broadcast "github.com/dustin/go-broadcast"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
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

func (p *kubeProxyClientStreams) resizeIn() remotecommand.TerminalSizeQueue {
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

func (p *party) Close() {
	p.closeOnce.Do(func() {
		p.Client.Close()
	})
}

// TODO(joel): handle transition to pending on leave
type session struct {
	mu sync.RWMutex

	ctx authContext

	forwarder *Forwarder

	req *http.Request

	id uuid.UUID

	parties map[string]*party

	partiesHistorical map[string]*party

	log *log.Entry

	clients_stdin *utils.TrackingReader

	clients_stdout *utils.TrackingWriter

	clients_stderr *utils.TrackingWriter

	terminalSizeQueue remotecommand.TerminalSizeQueue

	state types.SessionState

	stateUpdate broadcast.Broadcaster

	accessEvaluator auth.SessionAccessEvaluator

	recorder events.StreamWriter

	closeC chan struct{}

	closeOnce sync.Once
}

func newSession(ctx authContext, forwarder *Forwarder, req *http.Request, parentLog log.Entry) *session {
	id := uuid.New()
	// TODO(joel): supply roles
	accessEvaluator := auth.NewSessionAccessEvaluator(nil, types.KubernetesSessionKind)

	return &session{
		ctx:               ctx,
		forwarder:         forwarder,
		req:               req,
		id:                id,
		parties:           make(map[string]*party),
		partiesHistorical: make(map[string]*party),
		log:               log.WithField("session", id.String()),
		clients_stdin:     utils.NewTrackingReader(kubeutils.NewMultiReader()),
		clients_stdout:    utils.NewTrackingWriter(srv.NewMultiWriter()),
		clients_stderr:    utils.NewTrackingWriter(srv.NewMultiWriter()),
		state:             types.SessionState_SessionStatePending,
		stateUpdate:       broadcast.NewBroadcaster(1),
		accessEvaluator:   accessEvaluator,
	}
}

func (s *session) launch() error {
	sess, err := s.forwarder.newClusterSession(s.ctx)
	if err != nil {
		s.log.Errorf("Failed to create cluster session: %v.", err)
		return trace.Wrap(err)
	}

	executor, err := s.forwarder.getExecutor(s.ctx, sess, s.req)
	if err != nil {
		s.log.WithError(err).Warning("Failed creating executor.")
		return trace.Wrap(err)
	}

	options := remotecommand.StreamOptions{
		Stdin:             s.clients_stdin,
		Stdout:            s.clients_stdout,
		Stderr:            s.clients_stderr,
		Tty:               true,
		TerminalSizeQueue: s.terminalSizeQueue,
	}

	if err = executor.Stream(options); err != nil {
		s.log.WithError(err).Warning("Executor failed while streaming.")
		return trace.Wrap(err)
	}

	return nil
}

func (s *session) join(p *party) {
	panic("not implemented")
}

func (s *session) leave(id uuid.UUID) {
	panic("not implemented")
}

func (s *session) canStart() (bool, error) {
	// TODO(joel): supply participants
	yes, err := s.accessEvaluator.FulfilledFor(nil)
	return yes, trace.Wrap(err)
}

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
