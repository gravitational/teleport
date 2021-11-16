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
	"io"
	"sync"

	broadcast "github.com/dustin/go-broadcast"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
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

type session struct {
	mu sync.RWMutex

	parties map[string]*party

	partiesHistorical map[string]*party

	log *log.Entry

	writer utils.TrackingWriter

	reader utils.TrackingReader

	state types.SessionState

	stateUpdate broadcast.Broadcaster

	accessEvaluator auth.SessionAccessEvaluator

	recorder events.StreamWriter

	closeC chan struct{}
}

func newSession(ctx authContext, parentLog log.Entry) *session {
	id := uuid.New()

	return &session{
		parties:           make(map[string]*party),
		partiesHistorical: make(map[string]*party),
		log:               log.WithField("session", id.String()),
		state:             types.SessionState_SessionStatePending,
	}
}
