/*

 Copyright 2023 Gravitational, Inc.

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

package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/julienschmidt/httprouter"
	"github.com/sashabaranov/go-openai"

	"github.com/gravitational/teleport/api/client/proto"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
)

const (
	messageKindCommand                  = "COMMAND"
	messageKindCommandResult            = "COMMAND_RESULT"
	messageKindUserMessage              = "CHAT_MESSAGE_USER"
	messageKindAssistantMessage         = "CHAT_MESSAGE_ASSISTANT"
	messageKindAssistantPartialMessage  = "CHAT_PARTIAL_MESSAGE_ASSISTANT"
	messageKindAssistantPartialFinalize = "CHAT_PARTIAL_MESSAGE_ASSISTANT_FINALIZE"
	messageKindSystemMessage            = "CHAT_MESSAGE_SYSTEM"
)

func (h *Handler) createAssistantConversation(_ http.ResponseWriter, r *http.Request,
	_ httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := &proto.CreateAssistantConversationRequest{
		CreatedTime: h.clock.Now().UTC(),
	}

	resp, err := authClient.CreateAssistantConversation(r.Context(), req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (h *Handler) getAssistantConversationByID(_ http.ResponseWriter, r *http.Request,
	p httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conversationID := p.ByName("conversation_id")

	resp, err := authClient.GetAssistantMessages(r.Context(), conversationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type response struct {
		Messages []*proto.AssistantMessage
	}

	return &response{
		Messages: resp.Messages,
	}, err
}

// ConversationsResponse is a response for GET conversation response.
type ConversationsResponse struct {
	Conversations []*proto.ConversationInfo `json:"conversations"`
}

func (h *Handler) getAssistantConversations(_ http.ResponseWriter, r *http.Request,
	_ httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := authClient.GetAssistantConversations(r.Context(), &proto.GetAssistantConversationsRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.Conversations == nil {
		// If there are no conversations, return an empty array instead of null.
		resp.Conversations = []*proto.ConversationInfo{}
	}

	return &ConversationsResponse{
		Conversations: resp.Conversations,
	}, err
}

func (h *Handler) setAssistantTitle(_ http.ResponseWriter, r *http.Request,
	p httprouter.Params, sctx *SessionContext,
) (any, error) {
	req := struct {
		Title string `json:"title"`
	}{}

	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conversationID := p.ByName("conversation_id")

	conversationInfo := &proto.ConversationInfo{
		Id:    conversationID,
		Title: req.Title,
	}

	if err := authClient.SetAssistantConversationTitle(r.Context(), conversationInfo); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

func (h *Handler) generateAssistantTitle(_ http.ResponseWriter, r *http.Request,
	p httprouter.Params, sctx *SessionContext,
) (any, error) {
	req := struct {
		Message string `json:"message"`
	}{}

	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	chat, err := getAssistantClient(r.Context(), h.cfg.ProxyClient, h.cfg.ProxySettings, sctx.cfg.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	titleSummary, err := chat.Summary(r.Context(), req.Message)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conversationInfo := &proto.ConversationInfo{
		Title: titleSummary,
	}

	return conversationInfo, nil
}

func (h *Handler) assistant(w http.ResponseWriter, r *http.Request, _ httprouter.Params,
	sctx *SessionContext, site reversetunnel.RemoteSite,
) (any, error) {
	// moved into a separate function for error management/debug purposes
	if err := runAssistant(h, w, r, sctx, site); err != nil {
		h.log.Warn(trace.DebugReport(err))
		// The connection was very likely hijacked (when upgrading to websocket),
		// We should not try to send the error back. Writing to an hijacked
		// connection is illegal and will only cause more errors.
		return nil, nil
	}

	return nil, nil
}

type commandPayload struct {
	Command string     `json:"command,omitempty"`
	Nodes   []string   `json:"nodes,omitempty"`
	Labels  []ai.Label `json:"labels,omitempty"`
}

type partialMessagePayload struct {
	Content string `json:"content,omitempty"`
	Idx     int    `json:"idx,omitempty"`
}

type partialFinalizePayload struct {
	Idx int `json:"idx,omitempty"`
}

// runAssistant upgrades the HTTP connection to a websocket and starts a chat loop.
func runAssistant(h *Handler, w http.ResponseWriter, r *http.Request,
	sctx *SessionContext, site reversetunnel.RemoteSite,
) error {
	authClient, err := sctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	identity, err := createIdentityContext(sctx.GetUser(), sctx)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), identity, h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
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

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		errMsg := "Error upgrading to websocket"
		h.log.WithError(err).Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return nil
	}

	// Note: This time should be longer than OpenAI response time.
	keepAliveInterval := netConfig.GetKeepAliveInterval()
	err = ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
	if err != nil {
		h.log.WithError(err).Error("Error setting websocket readline")
		return nil
	}
	defer ws.Close()

	// Update the read deadline upon receiving a pong message.
	ws.SetPongHandler(func(_ string) error {
		ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
		return nil
	})

	ws.SetCloseHandler(func(code int, text string) error {
		h.log.Warnf("closing assistant websocket: %v %v", code, text)
		return nil
	})

	go startPingLoop(ctx, ws, keepAliveInterval, h.log, nil)

	chat, err := getAssistantClient(ctx, h.cfg.ProxyClient,
		h.cfg.ProxySettings, sctx.cfg.User)
	if err != nil {
		return trace.Wrap(err)
	}

	q := r.URL.Query()
	conversationID := q.Get("conversation_id")
	if conversationID == "" {
		return trace.BadParameter("conversation ID is required")
	}

	// existing conversation, retrieve old messages
	messages, err := authClient.GetAssistantMessages(ctx, conversationID)
	if err != nil {
		return trace.Wrap(err)
	}

	// restore conversation context.
	for _, msg := range messages.GetMessages() {
		role := kindToRole(msg.Type)
		if role != "" {
			chat.Insert(role, msg.Payload)
		}
	}

	if len(messages.GetMessages()) == 0 {
		// new conversation, generate hello message
		if _, err := processComplete(ctx, h, chat, conversationID, ws, authClient); err != nil {
			return trace.Wrap(err)
		}
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF || websocket.IsCloseError(
				err,
				websocket.CloseAbnormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived,
			) {
				break
			}
			return trace.Wrap(err)
		}

		var wsIncoming proto.AssistantMessage
		if err := json.Unmarshal(payload, &wsIncoming); err != nil {
			return trace.Wrap(err)
		}

		// write user message to both in-memory chain and persistent storage
		chat.Insert(openai.ChatMessageRoleUser, wsIncoming.Payload)

		promptTokens, err := chat.PromptTokens()
		if err != nil {
			log.Warnf("Failed to calculate prompt tokens: %v", err)
		}
		completionTokens, err := insertAssistantMessage(ctx, h, sctx, site, conversationID, wsIncoming, chat, ws)
		if err != nil {
			return trace.Wrap(err)
		}

		usageEventReq := &proto.SubmitUsageEventRequest{
			Event: &usageeventsv1.UsageEventOneOf{
				Event: &usageeventsv1.UsageEventOneOf_AssistCompletion{
					AssistCompletion: &usageeventsv1.AssistCompletionEvent{
						ConversationId:   conversationID,
						TotalTokens:      int64(promptTokens + completionTokens),
						PromptTokens:     int64(promptTokens),
						CompletionTokens: int64(completionTokens),
					},
				},
			},
		}
		if err := authClient.SubmitUsageEvent(r.Context(), usageEventReq); err != nil {
			h.log.WithError(err).Warn("Failed to emit usage event")
		}
	}

	h.log.Debugf("end assistant conversation loop")

	return nil
}

func insertAssistantMessage(ctx context.Context, h *Handler, sctx *SessionContext, site reversetunnel.RemoteSite,
	conversationID string, wsIncoming proto.AssistantMessage, chat *ai.Chat, ws *websocket.Conn,
) (int, error) {
	// Create a new auth client as the WS is a long-living connection
	// and client created at the beginning will timeout at some point.
	// TODO(jakule): Fix the timeout issue https://github.com/gravitational/teleport/issues/25758
	authClient, err := newRemoteClient(ctx, sctx, site)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer authClient.Close()

	if _, err := authClient.InsertAssistantMessage(ctx, &proto.AssistantMessage{
		ConversationId: conversationID,
		Type:           messageKindUserMessage,
		Payload:        wsIncoming.Payload,
		CreatedTime:    h.clock.Now().UTC(),
	}); err != nil {
		return 0, trace.Wrap(err)
	}

	numTokens, err := processComplete(ctx, h, chat, conversationID, ws, authClient)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return numTokens, nil
}

func getOpenAITokenFromDefaultPlugin(ctx context.Context, proxyClient auth.ClientI) (string, error) {
	// Try retrieving credentials from the plugin resource first
	openaiPlugin, err := proxyClient.PluginsClient().GetPlugin(ctx, &pluginsv1.GetPluginRequest{
		Name:        "openai-default",
		WithSecrets: true,
	})
	if err != nil {
		return "", trail.FromGRPC(err)
	}

	creds := openaiPlugin.Credentials.GetBearerToken()
	if creds == nil {
		return "", trace.BadParameter("malformed credentials")
	}
	if creds.TokenFile != "" {
		tokenBytes, err := os.ReadFile(creds.TokenFile)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return strings.TrimSpace(string(tokenBytes)), nil
	}

	return creds.Token, nil
}

func getAssistantClient(ctx context.Context, proxyClient auth.ClientI,
	proxySettings proxySettingsGetter, username string,
) (*ai.Chat, error) {
	token, err := getOpenAITokenFromDefaultPlugin(ctx, proxyClient)
	if err == nil {
		client := ai.NewClient(token)
		chat := client.NewChat(username)
		return chat, nil
	} else if !trace.IsNotFound(err) && !trace.IsNotImplemented(err) {
		// We ignore 2 types of errors here.
		// Unimplemented may be raised by the OSS server,
		// as PluginsService does not exist there yet.
		// NotFound means plugin does not exist,
		// in which case we should fall back on the static token configured in YAML.
		log.WithError(err).Error("Unexpected error fetching default OpenAI plugin")
	}

	keyGetter, found := proxySettings.(interface{ GetOpenAIAPIKey() string })
	if !found {
		return nil, trace.Errorf("GetOpenAIAPIKey is not implemented on %T", proxySettings)
	}

	apiKey := keyGetter.GetOpenAIAPIKey()
	if apiKey == "" {
		return nil, trace.Errorf("OpenAI API key is not set")
	}

	client := ai.NewClient(apiKey)
	chat := client.NewChat(username)

	return chat, nil
}

var jsonBlockPattern = regexp.MustCompile(`(?s){.+}`)

func tryFindEmbeddedCommand(message string) *ai.CompletionCommand {
	candidates := jsonBlockPattern.FindAllString(message, -1)

	for _, candidate := range candidates {
		var c ai.CompletionCommand
		if err := json.Unmarshal([]byte(candidate), &c); err == nil {
			return &c
		}
	}

	return nil
}

func processComplete(ctx context.Context, h *Handler, chat *ai.Chat, conversationID string,
	ws *websocket.Conn, authClient auth.ClientI,
) (int, error) {
	// query the assistant and fetch an answer
	message, numTokens, err := chat.Complete(ctx)
	if err != nil {
		return numTokens, trace.Wrap(err)
	}

	switch message := message.(type) {
	case *ai.StreamingMessage:
		// collection of the entire message, used for writing to conversation log
		content := ""

		// stream all chunks to client
	outer:
		for {
			select {
			case chunk, ok := <-message.Chunks:
				if !ok {
					break outer
				}

				if len(chunk) == 0 {
					continue outer
				}

				numTokens++
				content += chunk
				payload := partialMessagePayload{
					Content: chunk,
					Idx:     message.Idx,
				}

				payloadJSON, err := json.Marshal(payload)
				if err != nil {
					return numTokens, trace.Wrap(err)
				}

				protoMsg := &proto.AssistantMessage{
					ConversationId: conversationID,
					Type:           messageKindAssistantPartialMessage,
					Payload:        string(payloadJSON),
					CreatedTime:    h.clock.Now().UTC(),
				}

				if err := ws.WriteJSON(protoMsg); err != nil {
					return numTokens, trace.Wrap(err)
				}
			case err = <-message.Error:
				return numTokens, trace.Wrap(err)
			}
		}

		// tell the client that the message is complete
		finalizePayload := partialFinalizePayload{
			Idx: message.Idx,
		}

		finalizePayloadJSON, err := json.Marshal(finalizePayload)
		if err != nil {
			return numTokens, trace.Wrap(err)
		}

		finalizeProtoMsg := &proto.AssistantMessage{
			ConversationId: conversationID,
			Type:           messageKindAssistantPartialFinalize,
			Payload:        string(finalizePayloadJSON),
			CreatedTime:    h.clock.Now().UTC(),
		}

		if err := ws.WriteJSON(finalizeProtoMsg); err != nil {
			return numTokens, trace.Wrap(err)
		}

		// write the entire message to both in-memory chain and persistent storage
		chat.Insert(message.Role, content)
		protoMsg := &proto.AssistantMessage{
			ConversationId: conversationID,
			Type:           messageKindAssistantMessage,
			Payload:        content,
			CreatedTime:    h.clock.Now().UTC(),
		}

		if _, err := authClient.InsertAssistantMessage(ctx, protoMsg); err != nil {
			return numTokens, trace.Wrap(err)
		}

		// check if there's any embedded command in the response, if so, send a suggestion with it
		if command := tryFindEmbeddedCommand(content); command != nil {
			payload := commandPayload{
				Command: command.Command,
				Nodes:   command.Nodes,
				Labels:  command.Labels,
			}

			payloadJson, err := json.Marshal(payload)
			if err != nil {
				return numTokens, trace.Wrap(err)
			}

			msg := &proto.AssistantMessage{
				Type:           messageKindCommand,
				ConversationId: conversationID,
				Payload:        string(payloadJson),
				CreatedTime:    h.clock.Now().UTC(),
			}

			if _, err := authClient.InsertAssistantMessage(ctx, msg); err != nil {
				return numTokens, trace.Wrap(err)
			}

			if err := ws.WriteJSON(msg); err != nil {
				return numTokens, trace.Wrap(err)
			}
		}
	case *ai.Message:
		// write assistant message to both in-memory chain and persistent storage
		chat.Insert(message.Role, message.Content)
		protoMsg := &proto.AssistantMessage{
			ConversationId: conversationID,
			Type:           messageKindAssistantMessage,
			Payload:        message.Content,
			CreatedTime:    h.clock.Now().UTC(),
		}
		if _, err := authClient.InsertAssistantMessage(ctx, protoMsg); err != nil {
			return numTokens, trace.Wrap(err)
		}

		if err := ws.WriteJSON(protoMsg); err != nil {
			return numTokens, trace.Wrap(err)
		}
	case *ai.CompletionCommand:
		payload := commandPayload{
			Command: message.Command,
			Nodes:   message.Nodes,
			Labels:  message.Labels,
		}

		payloadJson, err := json.Marshal(payload)
		if err != nil {
			return numTokens, trace.Wrap(err)
		}

		msg := &proto.AssistantMessage{
			Type:           messageKindCommand,
			ConversationId: conversationID,
			Payload:        string(payloadJson),
			CreatedTime:    h.clock.Now().UTC(),
		}

		if _, err := authClient.InsertAssistantMessage(ctx, msg); err != nil {
			return numTokens, trace.Wrap(err)
		}

		if err := ws.WriteJSON(msg); err != nil {
			return numTokens, trace.Wrap(err)
		}
	default:
		return numTokens, trace.Errorf("unknown message type")
	}

	return numTokens, nil
}

func kindToRole(kind string) string {
	switch kind {
	case messageKindUserMessage:
		return openai.ChatMessageRoleUser
	case messageKindAssistantMessage:
		return openai.ChatMessageRoleAssistant
	case messageKindSystemMessage:
		return openai.ChatMessageRoleSystem
	default:
		return ""
	}
}
