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

package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	assistpb "github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/lib/ai/model/tools"
	"github.com/gravitational/teleport/lib/ai/tokens"
	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

const (
	// actionSSHGenerateCommand is a name of the action for generating SSH commands.
	actionSSHGenerateCommand = "ssh-cmdgen"
	// actionSSHExplainCommand is a name of the action for explaining terminal output in SSH session.
	actionSSHExplainCommand = "ssh-explain"
	// actionGenerateAuditQuery is the name of the action for generating audit queries.
	actionGenerateAuditQuery = "audit-query"
	// We cannot know how many tokens we will consume in advance.
	// Try to consume a small number of tokens first.
	lookaheadTokens = 100
)

// assistantMessage is an assistant message that is sent to the client.
type assistantMessage struct {
	// Type is a type of the message.
	Type assist.MessageType `json:"type"`
	// CreatedTime is a time when the message was created in RFC3339 format.
	CreatedTime string `json:"created_time"`
	// Payload is a message payload in JSON format.
	Payload string `json:"payload"`
}

// assistant is a handler for GET /webapi/sites/:site/assistant.
// This handler covers the main chat conversation as well as the
// SSH competition (SSH command generation and output explanation).
func (h *Handler) assistant(w http.ResponseWriter, r *http.Request, _ httprouter.Params,
	sctx *SessionContext, site reversetunnelclient.RemoteSite, ws *websocket.Conn,
) (any, error) {
	if err := runAssistant(h, w, r, sctx, site, ws); err != nil {
		h.log.Warn(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

// reserveTokens preemptively reserves tokens in the ratelimiter.
func (h *Handler) reserveTokens(usedTokens *tokens.TokenCount) (int, int) {
	promptTokens, completionTokens := usedTokens.CountAll()

	// Once we know how many tokens were consumed for prompt+completion,
	// consume the remaining tokens from the rate limiter bucket.
	extraTokens := promptTokens + completionTokens - lookaheadTokens
	if extraTokens < 0 {
		extraTokens = 0
	}
	h.assistantLimiter.ReserveN(time.Now(), extraTokens)
	return promptTokens, completionTokens
}

// reportTokenUsage sends a token usage event for an action.
func (h *Handler) reportActionTokenUsage(authClient auth.ClientI, usedTokens *tokens.TokenCount, action string) {
	// Create a new context to not be bounded by the request timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	promptTokens, completionTokens := h.reserveTokens(usedTokens)
	usageEventReq := &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AssistAction{
				AssistAction: &usageeventsv1.AssistAction{
					Action:           action,
					TotalTokens:      int64(promptTokens + completionTokens),
					PromptTokens:     int64(promptTokens),
					CompletionTokens: int64(completionTokens),
				},
			},
		},
	}

	if err := authClient.SubmitUsageEvent(ctx, usageEventReq); err != nil {
		h.log.WithError(err).Warn("Failed to emit usage event")
	}
}

func checkAssistEnabled(a auth.ClientI, ctx context.Context) error {
	enabled, err := a.IsAssistEnabled(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if !enabled.Enabled {
		return trace.AccessDenied("Assist is not enabled")
	}

	return nil
}

// runAssistant upgrades the HTTP connection to a websocket and starts a chat loop.
func runAssistant(h *Handler, w http.ResponseWriter, r *http.Request,
	sctx *SessionContext, site reversetunnelclient.RemoteSite, ws *websocket.Conn,
) (err error) {
	q := r.URL.Query()
	conversationID := q.Get("conversation_id")
	actionParam := r.URL.Query().Get("action")
	if conversationID == "" && actionParam == "" {
		return trace.BadParameter("conversation ID or action is required")
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return trace.Wrap(err)
	}

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), sctx, sctx.GetUser(), h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	authAccessPoint, err := site.CachingAccessPoint()
	if err != nil {
		h.log.WithError(err).Debug("Unable to get auth access point.")
		return trace.Wrap(err)
	}

	netConfig, err := authAccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		h.log.WithError(err).Debug("Unable to fetch cluster networking config.")
		return trace.Wrap(err)
	}

	// Note: This time should be longer than OpenAI response time.
	keepAliveInterval := netConfig.GetKeepAliveInterval()
	err = ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
	if err != nil {
		h.log.WithError(err).Error("Error setting websocket readline")
		return nil
	}
	defer func() {
		closureReason := websocket.CloseNormalClosure
		closureMsg := ""
		if err != nil {
			h.log.WithError(err).Error("Error in the Assistant loop")
			_ = ws.WriteJSON(&assistantMessage{
				Type:        assist.MessageKindError,
				Payload:     "An error has occurred. Please try again later.",
				CreatedTime: h.clock.Now().UTC().Format(time.RFC3339),
			})
			// Set server error code and message: https://datatracker.ietf.org/doc/html/rfc6455#section-7.4.1
			closureReason = websocket.CloseInternalServerErr
			closureMsg = err.Error()
		}
		// Send the close message to the client and close the connection
		if err := ws.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(closureReason, closureMsg),
			time.Now().Add(time.Second),
		); err != nil {
			h.log.Warnf("Failed to write close message: %v", err)
		}
		if err := ws.Close(); err != nil {
			h.log.Warnf("Failed to close websocket: %v", err)
		}
	}()

	// Update the read deadline upon receiving a pong message.
	ws.SetPongHandler(func(_ string) error {
		return trace.Wrap(ws.SetReadDeadline(deadlineForInterval(keepAliveInterval)))
	})

	ws.SetCloseHandler(func(code int, text string) error {
		h.log.Debugf("closing assistant websocket: %v %v", code, text)
		return nil
	})

	go startWSPingLoop(ctx, ws, keepAliveInterval, h.log, nil)

	assistClient, err := assist.NewClient(ctx, h.cfg.ProxyClient,
		h.cfg.ProxySettings, h.cfg.OpenAIConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	switch r.URL.Query().Get("action") {
	case actionSSHGenerateCommand:
		err = h.assistGenSSHCommandLoop(ctx, assistClient, authClient, ws, sctx.GetUser())
	case actionSSHExplainCommand:
		err = h.assistSSHExplainOutputLoop(ctx, assistClient, authClient, ws)
	case actionGenerateAuditQuery:
		err = h.assistGenAuditQueryLoop(ctx, assistClient, authClient, ws, sctx.GetUser())
	default:
		err = trace.Errorf("Teleport Assist Chat has been remove in v16")
	}

	return trace.Wrap(err)
}

// assistGenAuditQueryLoop reads the user's input and generates an audit query.
func (h *Handler) assistGenAuditQueryLoop(ctx context.Context, assistClient *assist.Assist, authClient auth.ClientI, ws *websocket.Conn, username string) error {
	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if wsIsClosed(err) {
				break
			}
			return trace.Wrap(err)
		}

		onMessage := func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
			return onMessageFn(ws, kind, payload, createdTime)
		}

		toolCtx := &tools.ToolContext{User: username}

		if err := h.preliminaryRateLimitGuard(onMessage); err != nil {
			return trace.Wrap(err)
		}

		tokenCount, err := assistClient.RunTool(ctx, onMessage, tools.AuditQueryGenerationToolName, string(payload), toolCtx)
		if err != nil {
			return trace.Wrap(err)
		}

		go h.reportActionTokenUsage(authClient, tokenCount, tools.AuditQueryGenerationToolName)
	}
	return nil
}

// assistSSHExplainOutputLoop reads the user's input and generates a command summary.
func (h *Handler) assistSSHExplainOutputLoop(ctx context.Context, assistClient *assist.Assist, authClient auth.ClientI, ws *websocket.Conn) error {
	_, payload, err := ws.ReadMessage()
	if err != nil {
		if wsIsClosed(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	modelMessages := []*assistpb.AssistantMessage{
		{
			Type:    string(assist.MessageKindUserMessage),
			Payload: string(payload),
		},
	}

	onMessage := func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
		return onMessageFn(ws, kind, payload, createdTime)
	}

	if err := h.preliminaryRateLimitGuard(onMessage); err != nil {
		return trace.Wrap(err)
	}

	summary, tokenCount, err := assistClient.GenerateCommandSummary(ctx, modelMessages, map[string][]byte{})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := onMessageFn(ws, assist.MessageKindAssistantMessage, []byte(summary), h.clock.Now().UTC()); err != nil {
		return trace.Wrap(err)
	}

	go h.reportActionTokenUsage(authClient, tokenCount, "SSH Explain")
	return nil
}

// assistSSHCommandLoop reads the user's input and generates a Linux command.
func (h *Handler) assistGenSSHCommandLoop(ctx context.Context, assistClient *assist.Assist, authClient auth.ClientI, ws *websocket.Conn, username string) error {
	chat, err := assistClient.NewLightweightChat(username)
	if err != nil {
		return trace.Wrap(err)
	}

	onMessage := func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
		return trace.Wrap(onMessageFn(ws, kind, payload, createdTime))
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if wsIsClosed(err) {
				break
			}
			return trace.Wrap(err)
		}

		if err := h.preliminaryRateLimitGuard(onMessage); err != nil {
			return trace.Wrap(err)
		}

		tokenCount, err := chat.ProcessComplete(ctx, func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
			return onMessageFn(ws, kind, payload, createdTime)
		}, string(payload))
		if err != nil {
			return trace.Wrap(err)
		}

		tool := tools.CommandExecutionTool{}
		go h.reportActionTokenUsage(authClient, tokenCount, tool.Name())
	}
	return nil
}

// preliminaryRateLimitGuard checks that some small number of tokens is still available and the ratelimit is not exceeded.
// This is done because the changed quantity within the limiter is not known until after a request is processed.
func (h *Handler) preliminaryRateLimitGuard(onMessageFn func(kind assist.MessageType, payload []byte, createdTime time.Time) error) error {
	const errorMsg = "You have reached the rate limit. Please try again later."

	if !h.assistantLimiter.AllowN(time.Now(), lookaheadTokens) {
		err := onMessageFn(assist.MessageKindError, []byte(errorMsg), h.clock.Now().UTC())
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.LimitExceeded(errorMsg)
	}

	return nil
}

// wsIsClosed returns true if the error is caused by a closed websocket.
func wsIsClosed(err error) bool {
	return errors.Is(err, io.EOF) || websocket.IsCloseError(err, websocket.CloseAbnormalClosure,
		websocket.CloseGoingAway, websocket.CloseNormalClosure)
}

// onMessageFn is a helper function used to send an assist message to the frontend.
// It deals with serializing the kind and payload into a wire and sending it over with the correct
// websocket frame type.
func onMessageFn(ws *websocket.Conn, kind assist.MessageType, payload []byte, createdTime time.Time) error {
	msg := &assistantMessage{
		Type:        kind,
		Payload:     string(payload),
		CreatedTime: createdTime.Format(time.RFC3339),
	}

	return trace.Wrap(ws.WriteJSON(msg))
}
