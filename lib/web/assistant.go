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
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sashabaranov/go-openai"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
)

const (
	kindChatTextMessage = "CHAT_TEXT_MESSAGE"
)

func (h *Handler) assistant(w http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext) (any, error) {
	err := runAssistant(h, w, r, sctx)
	if err != nil {
		h.log.Error(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func runAssistant(h *Handler, w http.ResponseWriter, r *http.Request, sctx *SessionContext) error {
	authClient, err := sctx.GetClient()
	if err != nil {
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

	keepAliveInterval := time.Minute * 5 // TODO(jakule)
	err = ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
	if err != nil {
		h.log.WithError(err).Error("Error setting websocket readline")
		return nil
	}
	defer ws.Close()

	prefs, err := h.cfg.ProxyClient.GetAuthPreference(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	client := ai.NewClient(prefs.(*types.AuthPreferenceV2).Spec.Assist.ApiKey)
	chat := client.NewChat()

	q := r.URL.Query()
	conversationID := q.Get("conversation_id")
	if conversationID == "" {
		// new conversation, create a new ID
		conversationID = uuid.New().String()

		if err := ws.WriteJSON(struct {
			ConversationID string `json:"conversation_id"`
		}{
			ConversationID: conversationID,
		}); err != nil {
			return trace.Wrap(err)
		}
	} else {
		// existing conversation, retrieve old messages
		messages, err := authClient.GetAssistantMessages(r.Context(), conversationID)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, msg := range messages.GetMessages() {
			var chatMsg chatMessage
			if err := json.Unmarshal(msg.Payload, &chatMsg); err != nil {
				return trace.Wrap(err)
			}

			msg := chat.Insert(chatMsg.Role, chatMsg.Content)
			if err := ws.WriteJSON(msg); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF {
				break
			}
			return trace.Wrap(err)
		}

		var wsIncoming inboundWsMessage
		if err := json.Unmarshal(payload, &wsIncoming); err != nil {
			return trace.Wrap(err)
		}

		chat.Insert(openai.ChatMessageRoleUser, wsIncoming.Content)
		msgJson, err := json.Marshal(chatMessage{Role: openai.ChatMessageRoleUser, Content: wsIncoming.Content})
		if _, err := authClient.InsertAssistantMessage(r.Context(), &proto.AssistantMessage{
			ConversationId: conversationID,
			Type:           kindChatTextMessage,
			Payload:        msgJson,
			CreatedTime:    h.clock.Now().UTC(),
		}); err != nil {
			return trace.Wrap(err)
		}

		message, err := chat.Complete(r.Context(), 500)
		if err != nil {
			return trace.Wrap(err)
		}

		msgJson, err = json.Marshal(chatMessage{Role: message.Role, Content: message.Content})
		if _, err := authClient.InsertAssistantMessage(r.Context(), &proto.AssistantMessage{
			ConversationId: conversationID,
			Type:           kindChatTextMessage,
			Payload:        msgJson,
			CreatedTime:    h.clock.Now().UTC(),
		}); err != nil {
			return trace.Wrap(err)
		}

		if err := ws.WriteJSON(message); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

type inboundWsMessage struct {
	Content string `json:"content"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
