/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package slack

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// interactionsBufferSize is the number of interaction events to buffer between Socket Mode client and consumer app.
	interactionsBufferSize = 10
	// reconnectBackoffBase is an initial (minimum) backoff value.
	reconnectBackoffBase = time.Millisecond
	// reconnectBackoffMax is a backoff threshold.
	reconnectBackoffMax = 5 * time.Second
	// reconnectAutoResetMultiplier is the multiplier applied to reconnectBackoffMax that determines
	// when the retry attempt counter should be reset.
	reconnectAutoResetMultiplier = 2
	// pingInterval is the interval to ping the Socket Mode server.
	pingInterval = 5 * time.Second
	// pingWriteWait is the write deadline for pings to the Socket Mode server.
	pingWriteWait = 3 * time.Second
	// pongWait is how long to wait for server pongs.
	pongWait = 4 * pingInterval
)

// ErrSocketModeDisconnect indicates that Slack Socket Mode server requests a disconnect
// and the WebSocket connection should be rebuilt.
var ErrSocketModeDisconnect = errors.New("slack requested disconnect")

// WebSocketURLGenerator provides a method to generate a WebSocket URL for Slack Socket Mode.
type WebSocketURLGenerator interface {
	GenerateWebSocketURL(ctx context.Context) (string, error)
}

// SocketModeClient is the client that receives app interaction events from the Slack Socket Mode server.
type SocketModeClient struct {
	interactionsCh chan InteractionEvent
	urlGenerator   WebSocketURLGenerator
}

// NewSocketModeClient creates a client to connect to Slack Socket Mode.
func NewSocketModeClient(urlGenerator WebSocketURLGenerator) *SocketModeClient {
	return &SocketModeClient{
		interactionsCh: make(chan InteractionEvent, interactionsBufferSize),
		urlGenerator:   urlGenerator,
	}
}

// Interactions returns the interaction events channel. Consumer apps should read from
// this channel and process Slack interactions for downstream logic.
func (smc *SocketModeClient) Interactions() <-chan InteractionEvent {
	return smc.interactionsCh
}

// Run starts the Socket Mode client to receive interaction events into the Interactions() channel.
// It handles automatic reconnects to the Socket Mode server.
// This is a blocking function that exits on fatal errors or if context is canceled.
func (smc *SocketModeClient) Run(ctx context.Context) error {
	log := logger.Get(ctx)

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver:    retryutils.NewExponentialDriver(reconnectBackoffBase),
		First:     reconnectBackoffBase,
		Max:       reconnectBackoffMax,
		Jitter:    retryutils.DefaultJitter,
		AutoReset: reconnectAutoResetMultiplier,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		err := smc.run(ctx)
		// We must exit on fatal connection errors (eg. invalid auth token to Slack).
		// Otherwise, reconnect after a backoff for transient errors or Slack disconnect requests.
		if err != nil && isFatalSocketModeError(trace.Unwrap(err)) {
			return trace.Wrap(err)
		}
		// Context is done, exit immediately.
		if ctx.Err() != nil {
			log.DebugContext(ctx, "Socket Mode client is canceled")
			return nil
		}

		log.DebugContext(ctx, "Disconnected from Socket Mode server, reconnecting...",
			"error", trace.UserMessageWithFields(err),
		)

		select {
		case <-retry.After():
			retry.Inc()
		case <-ctx.Done():
			log.DebugContext(ctx, "Socket Mode client is canceled")
			return nil
		}
	}
}

// run creates a WebSocket connection with the Socket Mode server and
// processes incoming Socket Mode events.
func (smc *SocketModeClient) run(ctx context.Context) error {
	log := logger.Get(ctx)

	ws, err := smc.connect(ctx)
	if err != nil {
		log.ErrorContext(ctx, "Failed to connect to Socket Mode server", "error", err)
		return trace.Wrap(err)
	}
	defer ws.Close()

	log.InfoContext(ctx, "Receiving Socket Mode events...")

	rawCh := make(chan json.RawMessage)
	ackCh := make(chan string)

	g, gctx := errgroup.WithContext(ctx)

	// Upon context cancellation, force close connection to unblock ws.ReadJSON.
	stop := context.AfterFunc(gctx, func() {
		_ = ws.Close()
	})
	defer stop()

	g.Go(func() error {
		return trace.Wrap(smc.ping(gctx, ws), "ping routine")
	})

	g.Go(func() error {
		return trace.Wrap(smc.parseEvents(gctx, rawCh, ackCh), "parseEvents routine")
	})

	g.Go(func() error {
		return trace.Wrap(smc.writePump(gctx, ws, ackCh), "writePump routine")
	})

	g.Go(func() error {
		return trace.Wrap(smc.readPump(gctx, ws, rawCh), "readPump routine")
	})

	return trace.Wrap(g.Wait())
}

// connect opens and dials a temporary WebSocket URL from the Slack API.
func (smc *SocketModeClient) connect(ctx context.Context) (*websocket.Conn, error) {
	log := logger.Get(ctx)

	socketModeURL, err := smc.urlGenerator.GenerateWebSocketURL(ctx)
	if err != nil {
		log.ErrorContext(ctx, "Failed to open a WebSocket URL for Socket Mode", "error", err)
		return nil, trace.Wrap(err)
	}

	ws, resp, err := websocket.DefaultDialer.DialContext(ctx, socketModeURL, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if errors.Is(err, websocket.ErrBadHandshake) {
			// resp is non-nil in this case, so we can read the http response body
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return nil, trace.Wrap(readErr)
			}
			return nil, trace.BadParameter("websocket bad handshake: %s", string(body))
		}
		return nil, trace.Wrap(err)
	}

	return ws, nil
}

// ping will periodically ping Socket Mode server to maintain connectivity.
func (smc *SocketModeClient) ping(ctx context.Context, ws *websocket.Conn) error {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(pingWriteWait)); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// readPump pumps incoming Socket Mode interaction events to the raw events channel.
func (smc *SocketModeClient) readPump(ctx context.Context, ws *websocket.Conn, rawCh chan<- json.RawMessage) error {
	log := logger.Get(ctx)

	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		var raw json.RawMessage
		err := ws.ReadJSON(&raw)
		if err != nil {
			// Only log unexpected network errors to avoid cluttering the logs.
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) &&
				!utils.IsOKNetworkError(err) {
				log.WarnContext(ctx, "websocket ReadJSON error", "error", err)
			}
			return trace.Wrap(err)
		}

		select {
		case rawCh <- raw:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// writePump writes ack messages back to the WebSocket connection.
func (smc *SocketModeClient) writePump(ctx context.Context, ws *websocket.Conn, ackCh <-chan string) error {
	log := logger.Get(ctx)
	for {
		select {
		case envelopeID := <-ackCh:
			err := ws.WriteJSON(SocketModeAckResponse{EnvelopeID: envelopeID})
			if err != nil {
				log.WarnContext(ctx, "websocket WriteJSON error", "error", err)
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// parseEvents parses events from the raw events channel and sends to the client Interactions() channel.
// It sends ack responses to the write pump.
func (smc *SocketModeClient) parseEvents(ctx context.Context, rawCh <-chan json.RawMessage, ackCh chan<- string) error {
	log := logger.Get(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case raw := <-rawCh:
			var e SocketModeEvent
			if err := json.Unmarshal(raw, &e); err != nil {
				log.WarnContext(ctx, "Error unmarshaling socket mode event, skipping...", "error", err, "raw_event", raw)
				continue
			}

			switch e.Type {
			case SocketModeEventTypeHello:
				log.DebugContext(ctx, "Received hello",
					"connection_info", e.ConnectionInfo,
					"num_connections", e.NumConnections,
					"debug_info", e.DebugInfo,
				)
			case SocketModeEventTypeDisconnect:
				log.DebugContext(ctx, "Received disconnect", "reason", e.Reason, "debug_info", e.DebugInfo)

				// Socket Mode is disabled in Slack app settings.
				if e.Reason == SocketModeEventReasonLinkDisabled {
					return trace.Errorf("%s", SocketModeEventReasonLinkDisabled)
				}

				// Socket Mode server wants to rebuild WebSocket connection.
				return trace.Wrap(ErrSocketModeDisconnect)
			case SocketModeEventTypeInteractive:
				log.DebugContext(ctx, "Received interaction event")

				// Send an ack to the write goroutine immediately.
				// Slack expects an ack within 3 seconds of interaction receipt.
				select {
				case ackCh <- e.EnvelopeID:
				case <-ctx.Done():
					return ctx.Err()
				}

				interaction, err := UnmarshalInteractionEvent(e.Payload)
				if err != nil {
					log.WarnContext(ctx, "Unsupported interaction event, skipping...", "error", err)
					continue
				}

				// Send to consumer app to handle interactions downstream.
				select {
				case smc.interactionsCh <- interaction:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

// isFatalSocketModeError returns true if the error is fatal for the Socket Mode client, eg. invalid app-level token or Socket Mode is disabled.
// See https://docs.slack.dev/reference/methods/apps.connections.open/ for generating a WebSocket URL and its error list.
// See https://docs.slack.dev/apis/events-api/using-socket-mode/#disconnect for Socket Mode disconnect messages.
func isFatalSocketModeError(err error) bool {
	switch err.Error() {
	case SocketModeEventReasonLinkDisabled:
		// Socket Mode is disabled in app settings.
		return true
	case "token_expired", "not_authed", "invalid_auth", "account_inactive", "token_revoked", "no_permission", "not_allowed_token_type", "team_access_not_granted", "missing_scope":
		// API authn/authz errors that are not retry-able.
		return true
	default:
		return false
	}
}
