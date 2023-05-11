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
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/julienschmidt/httprouter"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
)

// assistantMessageType is a type of the Assist message.
type assistantMessageType string

const (
	// messageKindCommand is the type of Assist message that contains the command to execute.
	messageKindCommand assistantMessageType = "COMMAND"
	// messageKindCommandResult is the type of Assist message that contains the command execution result.
	messageKindCommandResult assistantMessageType = "COMMAND_RESULT"
	// messageKindUserMessage is the type of Assist message that contains the user message.
	messageKindUserMessage assistantMessageType = "CHAT_MESSAGE_USER"
	// messageKindAssistantMessage is the type of Assist message that contains the assistant message.
	messageKindAssistantMessage assistantMessageType = "CHAT_MESSAGE_ASSISTANT"
	// messageKindAssistantPartialMessage is the type of Assist message that contains the assistant partial message.
	messageKindAssistantPartialMessage assistantMessageType = "CHAT_PARTIAL_MESSAGE_ASSISTANT"
	// messageKindAssistantPartialFinalize is the type of Assist message that ends the partial message stream.
	messageKindAssistantPartialFinalize assistantMessageType = "CHAT_PARTIAL_MESSAGE_ASSISTANT_FINALIZE"
	// messageKindSystemMessage is the type of Assist message that contains the system message.
	messageKindSystemMessage assistantMessageType = "CHAT_MESSAGE_SYSTEM"
)

// createAssistantConversationResponse is a response for POST /webapi/assistant/conversations.
type createdAssistantConversationResponse struct {
	// ID is a conversation ID.
	ID string `json:"id"`
}

// createAssistantConversation is a handler for POST /webapi/assistant/conversations.
func (h *Handler) createAssistantConversation(_ http.ResponseWriter, r *http.Request,
	_ httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := &assist.CreateAssistantConversationRequest{
		CreatedTime: timestamppb.New(h.clock.Now().UTC()),
		Username:    sctx.GetUser(),
	}

	resp, err := authClient.CreateAssistantConversation(r.Context(), req)
	if err != nil {
		return nil, err
	}

	return &createdAssistantConversationResponse{
		ID: resp.Id,
	}, nil
}

// assistantMessage is an assistant message that is sent to the client.
type assistantMessage struct {
	// Type is a type of the message.
	Type assistantMessageType `json:"type"`
	// CreatedTime is a time when the message was created in RFC3339 format.
	CreatedTime string `json:"created_time"`
	// Payload is a message payload in JSON format.
	Payload string `json:"payload"`
}

// getAssistantConversation is a handler for GET /webapi/assistant/conversations/:conversation_id.
func (h *Handler) getAssistantConversationByID(_ http.ResponseWriter, r *http.Request,
	p httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conversationID := p.ByName("conversation_id")

	resp, err := authClient.GetAssistantMessages(r.Context(), &assist.GetAssistantMessagesRequest{
		ConversationId: conversationID,
		Username:       sctx.GetUser(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conversationResponse(resp), nil
}

// conversationResponse creates a response for GET conversation response.
func conversationResponse(resp *assist.GetAssistantMessagesResponse) any {
	type response struct {
		Messages []assistantMessage `json:"messages"`
	}

	jsonResp := &response{
		Messages: make([]assistantMessage, 0, len(resp.Messages)),
	}

	for _, message := range resp.Messages {
		jsonResp.Messages = append(jsonResp.Messages, assistantMessage{
			Type:        assistantMessageType(message.Type),
			CreatedTime: message.CreatedTime.AsTime().Format(time.RFC3339),
			Payload:     message.Payload,
		})
	}

	return jsonResp
}

// conversationInfo is a response for GET conversation response.
type conversationInfo struct {
	// ID is a conversation ID.
	ID string `json:"id"`
	// Title is a conversation title.
	Title string `json:"title,omitempty"`
	// CreatedTime is a time when the conversation was created in RFC3339 format.
	CreatedTime string `json:"created_time"`
}

// conversationsResponse is a response for GET conversation response.
type conversationsResponse struct {
	Conversations []conversationInfo `json:"conversations"`
}

// getAssistantConversations is a handler for GET /webapi/assistant/conversations.
func (h *Handler) getAssistantConversations(_ http.ResponseWriter, r *http.Request,
	_ httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := authClient.GetAssistantConversations(r.Context(), &assist.GetAssistantConversationsRequest{
		Username: sctx.GetUser(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return genConversationsResponse(resp), nil
}

func genConversationsResponse(resp *assist.GetAssistantConversationsResponse) *conversationsResponse {
	jsonResp := &conversationsResponse{
		Conversations: make([]conversationInfo, 0, len(resp.Conversations)),
	}

	for _, conversation := range resp.Conversations {
		jsonResp.Conversations = append(jsonResp.Conversations, conversationInfo{
			ID:          conversation.Id,
			Title:       conversation.Title,
			CreatedTime: conversation.CreatedTime.AsTime().Format(time.RFC3339),
		})
	}

	return jsonResp
}

// setAssistantTitle is a handler for POST /webapi/assistant/conversations/:conversation_id/title.
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

	conversationInfo := &assist.UpdateAssistantConversationInfoRequest{
		ConversationId: conversationID,
		Username:       sctx.GetUser(),
		Title:          req.Title,
	}

	if err := authClient.UpdateAssistantConversationInfo(r.Context(), conversationInfo); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// generateAssistantTitleRequest is a request for POST /webapi/assistant/conversations/:conversation_id/generate_title.
type generateAssistantTitleRequest struct {
	Message string `json:"message"`
}

// generateAssistantTitle is a handler for POST /webapi/assistant/conversations/:conversation_id/generate_title.
func (h *Handler) generateAssistantTitle(_ http.ResponseWriter, r *http.Request,
	_ httprouter.Params, sctx *SessionContext,
) (any, error) {
	var req generateAssistantTitleRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := getAssistantClient(r.Context(), h.cfg.ProxyClient,
		h.cfg.ProxySettings, h.cfg.OpenAIConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chat := client.NewChat(sctx.cfg.User)

	titleSummary, err := chat.Summary(r.Context(), req.Message)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conversationInfo := &conversationInfo{
		Title: titleSummary,
	}

	return conversationInfo, nil
}

func (h *Handler) assistant(w http.ResponseWriter, r *http.Request, _ httprouter.Params,
	sctx *SessionContext, site reversetunnel.RemoteSite,
) (any, error) {
	if err := runAssistant(h, w, r, sctx, site); err != nil {
		h.log.Warn(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

// commandPayload is a payload for a command message.
type commandPayload struct {
	Command string     `json:"command,omitempty"`
	Nodes   []string   `json:"nodes,omitempty"`
	Labels  []ai.Label `json:"labels,omitempty"`
}

// partialMessagePayload is a payload for a partial message.
type partialMessagePayload struct {
	Content string `json:"content,omitempty"`
	Idx     int    `json:"idx,omitempty"`
}

// partialFinalizePayload is a payload for a partial finalize message.
type partialFinalizePayload struct {
	Idx int `json:"idx,omitempty"`
}

// runAssistant upgrades the HTTP connection to a websocket and starts a chat loop.
func runAssistant(h *Handler, w http.ResponseWriter, r *http.Request,
	sctx *SessionContext, site reversetunnel.RemoteSite,
) error {
	q := r.URL.Query()
	conversationID := q.Get("conversation_id")
	if conversationID == "" {
		return trace.BadParameter("conversation ID is required")
	}

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

	client, err := getAssistantClient(ctx, h.cfg.ProxyClient,
		h.cfg.ProxySettings, h.cfg.OpenAIConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	chat := client.NewChat(sctx.cfg.User)

	// existing conversation, retrieve old messages
	messages, err := authClient.GetAssistantMessages(ctx, &assist.GetAssistantMessagesRequest{
		ConversationId: conversationID,
		Username:       sctx.GetUser(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// restore conversation context.
	for _, msg := range messages.GetMessages() {
		role := kindToRole(assistantMessageType(msg.Type))
		if role != "" {
			chat.Insert(role, msg.Payload)
		}
	}

	if len(messages.GetMessages()) == 0 {
		// new conversation, generate a hello message
		if _, err := processComplete(ctx, h, chat, conversationID, sctx.GetUser(), ws, authClient); err != nil {
			return trace.Wrap(err)
		}
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF || websocket.IsCloseError(err, websocket.CloseAbnormalClosure,
				websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				break
			}
			return trace.Wrap(err)
		}

		var wsIncoming assistantMessage
		if err := json.Unmarshal(payload, &wsIncoming); err != nil {
			return trace.Wrap(err)
		}

		//TODO(jakule): Should we sanitize the payload?
		// write a user message to both an in-memory chain and persistent storage
		chat.Insert(openai.ChatMessageRoleUser, wsIncoming.Payload)

		promptTokens, err := chat.PromptTokens()
		if err != nil {
			log.Warnf("Failed to calculate prompt tokens: %v", err)
		}
		completionTokens, err := insertAssistantMessage(ctx, h, sctx, site,
			conversationID, &wsIncoming, chat, ws)
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

// insertAssistantMessage inserts a message from the user into the conversation.
// It returns the number of tokens in the completion response.
func insertAssistantMessage(ctx context.Context, h *Handler, sctx *SessionContext, site reversetunnel.RemoteSite,
	conversationID string, wsIncoming *assistantMessage, chat *ai.Chat, ws *websocket.Conn,
) (int, error) {
	// Create a new auth client as the WS is a long-living connection
	// and client created at the beginning will timeout at some point.
	// TODO(jakule): Fix the timeout issue https://github.com/gravitational/teleport/issues/25758
	authClient, err := newRemoteClient(ctx, sctx, site)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer authClient.Close()

	if err := authClient.CreateAssistantMessage(ctx, &assist.CreateAssistantMessageRequest{
		Message: &assist.AssistantMessage{
			Type:        string(messageKindUserMessage),
			Payload:     wsIncoming.Payload, // TODO(jakule): Sanitize the payload
			CreatedTime: timestamppb.New(h.clock.Now().UTC()),
		},
		ConversationId: conversationID,
		Username:       sctx.GetUser(),
	}); err != nil {
		return 0, trace.Wrap(err)
	}

	numTokens, err := processComplete(ctx, h, chat, conversationID, sctx.GetUser(), ws, authClient)
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
	proxySettings proxySettingsGetter, openaiCfg *openai.ClientConfig,
) (*ai.Client, error) {
	token, err := getOpenAITokenFromDefaultPlugin(ctx, proxyClient)
	if err == nil {
		return ai.NewClient(token), nil
	} else if !trace.IsNotFound(err) && !trace.IsNotImplemented(err) {
		// We ignore 2 types of errors here.
		// Unimplemented may be raised by the OSS server,
		// as PluginsService does not exist there yet.
		// NotFound means plugin does not exist,
		// in which case we should fall back on the static token configured in YAML.
		log.WithError(err).Error("Unexpected error fetching default OpenAI plugin")
	}

	// If the default plugin is not configured, try to get the token from the proxy settings.
	keyGetter, found := proxySettings.(interface{ GetOpenAIAPIKey() string })
	if !found {
		return nil, trace.Errorf("GetOpenAIAPIKey is not implemented on %T", proxySettings)
	}

	apiKey := keyGetter.GetOpenAIAPIKey()
	if apiKey == "" {
		return nil, trace.Errorf("OpenAI API key is not set")
	}

	// Allow using the passed config if passed.
	if openaiCfg != nil {
		return ai.NewClientFromConfig(*openaiCfg), nil
	}
	return ai.NewClient(token), nil
}

var jsonBlockPattern = regexp.MustCompile(`(?s){.+}`)

// tryFindEmbeddedCommand tries to find an embedded command in the message.
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

func processComplete(ctx context.Context, h *Handler, chat *ai.Chat,
	conversationID, username string,
	ws *websocket.Conn, authClient auth.ClientI,
) (int, error) {
	var numTokens int

	// query the assistant and fetch an answer
	message, err := chat.Complete(ctx)
	if err != nil {
		return numTokens, trace.Wrap(err)
	}

	switch message := message.(type) {
	case *ai.StreamingMessage:
		// collection of the entire message, used for writing to conversation log
		content := ""

		// stream all chunks to the client
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

				protoMsg := &assistantMessage{
					Type:        messageKindAssistantPartialMessage,
					Payload:     string(payloadJSON),
					CreatedTime: h.clock.Now().UTC().Format(time.RFC3339),
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

		finalizeProtoMsg := &assistantMessage{
			Type:        messageKindAssistantPartialFinalize,
			Payload:     string(finalizePayloadJSON),
			CreatedTime: h.clock.Now().UTC().Format(time.RFC3339),
		}

		if err := ws.WriteJSON(finalizeProtoMsg); err != nil {
			return numTokens, trace.Wrap(err)
		}

		// write the entire message to both an in-memory chain and persistent storage
		chat.Insert(message.Role, content)
		protoMsg := &assist.CreateAssistantMessageRequest{
			ConversationId: conversationID,
			Username:       username,
			Message: &assist.AssistantMessage{
				Type:        string(messageKindAssistantMessage),
				Payload:     content,
				CreatedTime: timestamppb.New(h.clock.Now().UTC()),
			},
		}

		if err := authClient.CreateAssistantMessage(ctx, protoMsg); err != nil {
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

			msg := &assist.CreateAssistantMessageRequest{
				ConversationId: conversationID,
				Username:       username,
				Message: &assist.AssistantMessage{
					Type:        string(messageKindCommand),
					Payload:     string(payloadJson),
					CreatedTime: timestamppb.New(h.clock.Now().UTC()),
				},
			}

			if err := authClient.CreateAssistantMessage(ctx, msg); err != nil {
				return numTokens, trace.Wrap(err)
			}

			if err := ws.WriteJSON(msg.Message); err != nil {
				return numTokens, trace.Wrap(err)
			}
		}
	case *ai.Message:
		numTokens = message.NumTokens
		// write an assistant message to both an in-memory chain and persistent storage
		chat.Insert(message.Role, message.Content)
		protoMsg := &assist.CreateAssistantMessageRequest{
			ConversationId: conversationID,
			Username:       username,
			Message: &assist.AssistantMessage{
				Type:        string(messageKindAssistantMessage),
				Payload:     message.Content,
				CreatedTime: timestamppb.New(h.clock.Now().UTC()),
			},
		}

		if err := authClient.CreateAssistantMessage(ctx, protoMsg); err != nil {
			return numTokens, trace.Wrap(err)
		}

		jsonPayload := &assistantMessage{
			Type:        messageKindAssistantMessage,
			Payload:     message.Content,
			CreatedTime: h.clock.Now().UTC().Format(time.RFC3339),
		}

		if err := ws.WriteJSON(jsonPayload); err != nil {
			return numTokens, trace.Wrap(err)
		}
	case *ai.CompletionCommand:
		numTokens = message.NumTokens
		payload := commandPayload{
			Command: message.Command,
			Nodes:   message.Nodes,
			Labels:  message.Labels,
		}

		payloadJson, err := json.Marshal(payload)
		if err != nil {
			return numTokens, trace.Wrap(err)
		}

		msg := &assist.CreateAssistantMessageRequest{
			ConversationId: conversationID,
			Username:       username,
			Message: &assist.AssistantMessage{
				Type:        string(messageKindCommand),
				Payload:     string(payloadJson),
				CreatedTime: timestamppb.New(h.clock.Now().UTC()),
			},
		}

		if err := authClient.CreateAssistantMessage(ctx, msg); err != nil {
			return numTokens, trace.Wrap(err)
		}

		jsonPayload := &assistantMessage{
			Type:        messageKindCommand,
			Payload:     string(payloadJson),
			CreatedTime: h.clock.Now().UTC().Format(time.RFC3339),
		}

		if err := ws.WriteJSON(jsonPayload); err != nil {
			return numTokens, trace.Wrap(err)
		}
	default:
		return numTokens, trace.Errorf("unknown message type")
	}

	return numTokens, nil
}

func kindToRole(kind assistantMessageType) string {
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
