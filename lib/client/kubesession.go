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
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
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

// KubeSessionConfig contains configuration parameters used to join
// an existing Kubernetes session by [NewKubeSession].
type KubeSessionConfig struct {
	KubeProxyAddr                 string
	WebProxyAddr                  string
	TLSRoutingConnUpgradeRequired bool
	EnableEscapeSequences         bool
	Tracker                       types.SessionTracker
	TLSConfig                     *tls.Config
	Mode                          types.SessionParticipantMode
	AuthClient                    func(context.Context) (authclient.ClientI, error)
	Ceremony                      *mfa.Ceremony
	Stdin                         io.Reader
	Stdout                        io.Writer
	Stderr                        io.Writer
}

// NewKubeSession joins a live kubernetes session.
func NewKubeSession(ctx context.Context, cfg KubeSessionConfig) (*KubeSession, error) {
	ctx, cancel := context.WithCancel(ctx)
	joinEndpoint := "wss://" + cfg.KubeProxyAddr + "/api/v1/teleport/join/" + cfg.Tracker.GetSessionID()

	dialer := &websocket.Dialer{
		NetDialContext:  kubeSessionNetDialer(ctx, cfg).DialContext,
		TLSClientConfig: cfg.TLSConfig,
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
	defer func() {
		if err == nil {
			return
		}

		if err := ws.Close(); err != nil {
			log.DebugContext(ctx, "Close stream in response to context termination", "error", err)
		}
	}()

	stream, err := streamproto.NewSessionStream(ws, streamproto.ClientHandshake{Mode: cfg.Mode})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	context.AfterFunc(ctx, func() {
		_ = stream.Close()
	})

	term, err := terminal.New(cfg.Stdin, cfg.Stdout, cfg.Stderr)
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

	go handleResizeEvents(ctx, stream, term)

	s := &KubeSession{stream, term, ctx, cancel, cfg.Tracker, sync.WaitGroup{}}
	if err := s.handleMFA(ctx, cfg.AuthClient, cfg.Ceremony, cfg.Mode, stdout); err != nil {
		return nil, trace.Wrap(err)
	}

	s.pipeInOut(ctx, stdout, cfg.EnableEscapeSequences, cfg.Mode)
	return s, nil
}

func kubeSessionNetDialer(ctx context.Context, cfg KubeSessionConfig) client.ContextDialer {
	dialOpts := []client.DialOption{
		client.WithInsecureSkipVerify(cfg.TLSConfig.InsecureSkipVerify),
	}

	// Add options for ALPN connection upgrade only if kube is served at Proxy
	// web address.
	if cfg.WebProxyAddr == cfg.KubeProxyAddr && cfg.TLSRoutingConnUpgradeRequired {
		dialOpts = append(dialOpts,
			client.WithALPNConnUpgrade(cfg.TLSRoutingConnUpgradeRequired),
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

func handleResizeEvents(ctx context.Context, stream *streamproto.SessionStream, term *terminal.Terminal) {
	streamResizes := stream.ResizeQueue()
	terminalResizes := term.Subscribe()
	defer stream.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case size, more := <-streamResizes:
			if !more {
				return
			}
			if size == nil {
				continue
			}
			if err := term.Resize(int16(size.Width), int16(size.Height)); err != nil {
				fmt.Printf("Error attempting to resize terminal: %v\n\r", err)
			}
		case event, more := <-terminalResizes:
			if !more {
				return
			}
			_, ok := event.(terminal.ResizeEvent)
			if ok {
				w, h, err := term.Size()
				if err != nil {
					fmt.Printf("Error attempting to fetch terminal size: %v\n\r", err)
				}

				size := remotecommand.TerminalSize{Width: uint16(w), Height: uint16(h)}
				if err := stream.Resize(&size); err != nil {
					fmt.Printf("Error attempting to resize terminal: %v\n\r", err)
				}
			}
		}
	}
}

func (s *KubeSession) handleMFA(ctx context.Context, authFn func(context.Context) (authclient.ClientI, error), ceremony *mfa.Ceremony, mode types.SessionParticipantMode, stdout io.Writer) error {
	if s.stream.MFARequired && mode == types.SessionModeratorMode {
		auth, err := authFn(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		go func() {
			defer auth.Close()
			if err := RunPresenceTask(ctx, stdout, auth, s.meta.GetSessionID(), ceremony); err != nil {
				slog.DebugContext(ctx, "Presence check terminated unexpectedly", "error", err)
			}
		}()
	}

	return nil
}

// pipeInOut starts background tasks that copy input to and from the terminal.
func (s *KubeSession) pipeInOut(ctx context.Context, stdout io.Writer, enableEscapeSequences bool, mode types.SessionParticipantMode) {
	// wait for the session to copy everything
	s.wg.Add(1)
	go func() {
		defer func() {
			s.wg.Done()
			s.cancel()
		}()
		if _, err := io.Copy(stdout, s.stream); err != nil {
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
				if err := s.stream.ForceTerminate(); err != nil {
					log.DebugContext(ctx, "Error sending force termination request", "error", err)
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
