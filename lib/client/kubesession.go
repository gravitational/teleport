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

package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeSession a joined kubernetes session from the client side.
type KubeSession struct {
	stream *streamproto.SessionStream
	term   *terminal.Terminal
	ctx    context.Context
	cancel context.CancelFunc
	meta   types.SessionTracker
	wg     sync.WaitGroup
}

// NewKubeSession joins a live kubernetes session.
func NewKubeSession(ctx context.Context, tc *TeleportClient, meta types.SessionTracker, kubeAddr string, tlsServer string, mode types.SessionParticipantMode, tlsConfig *tls.Config) (*KubeSession, error) {
	ctx, cancel := context.WithCancel(ctx)
	joinEndpoint := "wss://" + kubeAddr + "/api/v1/teleport/join/" + meta.GetSessionID()

	if tlsServer != "" {
		tlsConfig.ServerName = tlsServer
	}

	dialer := &websocket.Dialer{
		NetDialContext:  kubeSessionNetDialer(ctx, tc, kubeAddr).DialContext,
		TLSClientConfig: tlsConfig,
	}

	ws, resp, err := dialer.DialContext(ctx, joinEndpoint, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		cancel()
		if resp == nil || resp.Body == nil {
			return nil, trace.Wrap(err)
		}

		body, _ := io.ReadAll(resp.Body)
		var respData map[string]interface{}
		if err := json.Unmarshal(body, &respData); err != nil {
			return nil, trace.Wrap(err)
		}

		if message, ok := respData["message"]; ok {
			if message, ok := message.(string); ok {
				return nil, trace.Errorf("%v", message)
			}
		}

		return nil, trace.BadParameter("failed to decode remote error: %v", string(body))
	}

	stream, err := streamproto.NewSessionStream(ws, streamproto.ClientHandshake{Mode: mode})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	term, err := terminal.New(tc.Stdin, tc.Stdout, tc.Stderr)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	if term.IsAttached() {
		// Put the terminal into raw mode. Note that this must be done before
		// pipeInOut() as it may replace streams.
		term.InitRaw(true)
	}

	stdout := utils.NewSyncWriter(term.Stdout())

	go handleOutgoingResizeEvents(ctx, stream, term)
	go handleIncomingResizeEvents(stream, term)

	s := &KubeSession{stream, term, ctx, cancel, meta, sync.WaitGroup{}}
	err = s.handleMFA(ctx, tc, mode, stdout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.pipeInOut(stdout, tc.EnableEscapeSequences, mode)
	return s, nil
}

func kubeSessionNetDialer(ctx context.Context, tc *TeleportClient, kubeAddr string) client.ContextDialer {
	dialOpts := []client.DialOption{
		client.WithInsecureSkipVerify(tc.InsecureSkipVerify),
	}

	// Add options for ALPN connection upgrade only if kube is served at Proxy
	// web address.
	if tc.WebProxyAddr == kubeAddr && tc.TLSRoutingConnUpgradeRequired {
		dialOpts = append(dialOpts,
			client.WithALPNConnUpgrade(tc.TLSRoutingConnUpgradeRequired),
			client.WithALPNConnUpgradePing(true), // Use Ping protocol for long-lived connections.
		)
	}

	return client.NewDialer(
		ctx,
		defaults.DefaultIdleTimeout,
		defaults.DefaultIOTimeout,
		dialOpts...,
	)
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
		clt, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		auth, err := clt.ConnectToCluster(ctx, s.meta.GetClusterName())
		if err != nil {
			return trace.Wrap(err)
		}

		go func() {
			RunPresenceTask(ctx, stdout, auth, s.meta.GetSessionID(), tc.NewMFACeremony())
			auth.Close()
			clt.Close()
		}()
	}

	return nil
}

// pipeInOut starts background tasks that copy input to and from the terminal.
func (s *KubeSession) pipeInOut(stdout io.Writer, enableEscapeSequences bool, mode types.SessionParticipantMode) {
	// wait for the session to copy everything
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
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
			handlePeerControls(s.term, enableEscapeSequences, s.stream)
		default:
			handleNonPeerControls(mode, s.term, func() {
				err := s.stream.ForceTerminate()
				if err != nil {
					log.DebugContext(context.Background(), "Error sending force termination request", "error", err)
					fmt.Print("\n\rError while sending force termination request\n\r")
				}
			})
		}
	}()
}

// Wait waits for the session to finish.
func (s *KubeSession) Wait() {
	// Wait for the session to copy everything into stdout
	s.wg.Wait()
}

// Close sends a close request to the other end and waits it to gracefully terminate the connection.
func (s *KubeSession) Close() error {
	if err := s.stream.Close(); err != nil {
		return trace.Wrap(err)
	}

	s.wg.Wait()
	return trace.Wrap(s.Detach())
}

// Detach detaches the terminal from the session. Must be called if Close is not called.
func (s *KubeSession) Detach() error {
	return trace.Wrap(s.term.Close())
}
