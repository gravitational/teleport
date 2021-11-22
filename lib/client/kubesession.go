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

package client

import (
	"context"
	"io"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type KubeSession struct {
	stream    *streamproto.SessionStream
	terminal  *terminal.Terminal
	close     *utils.CloseBroadcaster
	closeWait *sync.WaitGroup
}

func NewKubeSession(ctx context.Context, tc *TeleportClient, meta types.Session, key *Key) (*KubeSession, error) {
	close := utils.NewCloseBroadcaster()
	closeWait := &sync.WaitGroup{}

	if _, err := tc.Ping(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if tc.KubeProxyAddr == "" {
		// Kubernetes support disabled, don't touch kubeconfig.
		return nil, trace.AccessDenied("this cluster does not support kubernetes")
	}

	joinEndpoint := tc.KubeProxyAddr + "/api/teleport/v1/join/" + meta.GetID()

	// TODO(joel): deal with TLS routing
	//if tc.TLSRoutingEnabled {
	//	kubeStatus.tlsServerName = getKubeTLSServerName(tc)
	//}

	// TODO(joel): use correct credentials
	ws, _, err := websocket.DefaultDialer.Dial(joinEndpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(joel): set correct terminal size and deal with terminal resizing

	stream := streamproto.NewSessionStream(ws)

	terminal, err := terminal.New(tc.Stdin, tc.Stdout, tc.Stderr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeWait.Add(1)
	defer func() {
		terminal.Close()
		closeWait.Done()
		close.Close()
	}()

	if terminal.IsAttached() {
		// Put the terminal into raw mode. Note that this must be done before
		// pipeInOut() as it may replace streams.
		terminal.InitRaw(true)
	}

	s := &KubeSession{stream, terminal, close, closeWait}
	s.pipeInOut()
	return s, nil
}

func (s *KubeSession) pipeInOut() {
	go func() {
		defer s.close.Close()
		_, err := io.Copy(s.terminal.Stdout(), s.stream)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	s.closeWait.Add(1)
	go func() {
		defer s.closeWait.Done()
		defer s.close.Close()
	}()
}

func (s *KubeSession) Wait() {
	s.closeWait.Wait()
}

func (s *KubeSession) Close() {
	s.close.Close()
	s.closeWait.Wait()
}
