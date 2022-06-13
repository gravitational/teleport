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
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/remotecommand"
)

const mfaChallengeInterval = time.Second * 30

// KubeSession a joined kubernetes session from the client side.
type KubeSession struct {
	stream     *streamproto.SessionStream
	term       *terminal.Terminal
	ctx        context.Context
	cancelFunc context.CancelFunc
	cancelOnce sync.Once
	closeWait  *sync.WaitGroup
	meta       types.SessionTracker
}

// NewKubeSession joins a live kubernetes session.
func NewKubeSession(ctx context.Context, tc *TeleportClient, meta types.SessionTracker, kubeAddr string, tlsServer string, mode types.SessionParticipantMode, tlsConfig *tls.Config) (*KubeSession, error) {
	closeWait := &sync.WaitGroup{}
	joinEndpoint := "wss://" + kubeAddr + "/api/v1/teleport/join/" + meta.GetSessionID()

	if tlsServer != "" {
		tlsConfig.ServerName = tlsServer
	}

	dialer := &websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	ws, resp, err := dialer.Dial(joinEndpoint, nil)
	defer resp.Body.Close()
	if err != nil {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Handshake failed with status %d\nand body: %v\n", resp.StatusCode, string(body))
		return nil, trace.Wrap(err)
	}

	stream, err := streamproto.NewSessionStream(ws, streamproto.ClientHandshake{Mode: mode})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	term, err := terminal.New(tc.Stdin, tc.Stdout, tc.Stderr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	closeWait.Add(1)
	go func() {
		<-ctx.Done()
		term.Close()
		closeWait.Done()
	}()

	if term.IsAttached() {
		// Put the terminal into raw mode. Note that this must be done before
		// pipeInOut() as it may replace streams.
		term.InitRaw(true)
	}

	stdout := utils.NewSyncWriter(term.Stdout())

	go func() {
		handleOutgoingResizeEvents(ctx, stream, term)
	}()

	closeWait.Add(1)
	go func() {
		handleIncomingResizeEvents(stream, term)
		closeWait.Done()
	}()

	s := &KubeSession{stream: stream, term: term, ctx: ctx, cancelFunc: cancel, closeWait: closeWait, meta: meta}
	err = s.handleMFA(ctx, tc, mode, stdout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.pipeInOut(stdout, mode)
	return s, nil
}

func (s *KubeSession) cancel() {
	s.cancelOnce.Do(func() {
		s.cancelFunc()
	})
}

func handleOutgoingResizeEvents(ctx context.Context, stream *streamproto.SessionStream, term *terminal.Terminal) {
	queue := stream.ResizeQueue()

	select {
	case <-ctx.Done():
		return
	case size := <-queue:
		if size == nil {
			return
		}

		term.Resize(int16(size.Width), int16(size.Height))
	}
}

func handleIncomingResizeEvents(stream *streamproto.SessionStream, term *terminal.Terminal) {
	events := term.Subscribe()

	for {
		event, more := <-events
		_, ok := event.(terminal.ResizeEvent)
		if ok {
			w, h, err := term.Size()
			if err != nil {
				fmt.Printf("Error attempting to fetch terminal size: %v\n\r", err)
			}

			size := remotecommand.TerminalSize{Width: uint16(w), Height: uint16(h)}
			err = stream.Resize(&size)
			if err != nil {
				fmt.Printf("Error attempting to resize terminal: %v\n\r", err)
			}
		}

		if !more {
			break
		}
	}
}

func (s *KubeSession) handleMFA(ctx context.Context, tc *TeleportClient, mode types.SessionParticipantMode, stdout io.Writer) error {
	if s.stream.MFARequired && mode == types.SessionModeratorMode {
		proxy, err := tc.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		auth, err := proxy.ConnectToCluster(ctx, s.meta.GetClusterName())
		if err != nil {
			return trace.Wrap(err)
		}

		subCtx, cancel := context.WithCancel(ctx)
		go func() {
			<-ctx.Done()
			cancel()
		}()

		go runPresenceTask(subCtx, stdout, auth, tc, s.meta.GetSessionID())
	}

	return nil
}

// pipeInOut starts background tasks that copy input to and from the terminal.
func (s *KubeSession) pipeInOut(stdout io.Writer, mode types.SessionParticipantMode) {
	go func() {
		defer s.cancel()
		_, err := io.Copy(stdout, s.stream)
		if err != nil {
			fmt.Printf("Error while reading remote stream: %v\n\r", err.Error())
		}
	}()

	go func() {
		defer s.cancel()

		switch mode {
		case types.SessionPeerMode:
			handlePeerControls(s.term, s.stream)
		default:
			handleNonPeerControls(mode, s.term, func() {
				err := s.stream.ForceTerminate()
				if err != nil {
					log.Debugf("Error sending force termination request: %v", err)
					fmt.Print("\n\rError while sending force termination request\n\r")
				}
			})
		}
	}()
}

// Wait waits for the session to finish.
func (s *KubeSession) Wait() {
	s.closeWait.Wait()
}

// Close sends a close request to the other end and waits it to gracefully terminate the connection.
func (s *KubeSession) Close() {
	s.cancel()
	s.closeWait.Wait()
}
