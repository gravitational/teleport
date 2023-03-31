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
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
)

func (h *Handler) assistant(w http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
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
		return nil, nil
	}

	keepAliveInterval := time.Minute // TODO(jakule)
	err = ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
	if err != nil {
		h.log.WithError(err).Error("Error setting websocket readline")
		return nil, nil
	}
	defer ws.Close()

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
			return nil, trace.Wrap(err)
		}
	} else {
		// existing conversation, retrieve old messages
		messages, err := authClient.GetAssistantMessages(r.Context(), conversationID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, msg := range messages.GetMessages() {
			if err := ws.WriteJSON(msg); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, trace.Wrap(err)
		}

		// TODO: implement ChatGPT communication

		if _, err := authClient.InsertAssistantMessage(r.Context(), &proto.AssistantMessage{
			ConversationId: conversationID,
			Type:           "CHAT_RESPONSE", // Set the message type
			Payload:        msg,
		}); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return nil, nil
}
