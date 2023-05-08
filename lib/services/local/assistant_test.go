/*
 *
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package local_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
)

// TestAssistantCRUD tests the assistant CRUD operations.
func TestAssistantCRUD(t *testing.T) {
	t.Parallel()

	identity := newIdentityService(t, clockwork.NewFakeClock())
	ctx := context.Background()

	const username = "foo"
	var conversationID string

	t.Run("create conversation", func(t *testing.T) {
		req := &proto.CreateAssistantConversationRequest{
			Username:    username,
			CreatedTime: time.Now(),
		}

		conversationResp, err := identity.CreateAssistantConversation(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, conversationResp.Id)

		conversationID = conversationResp.Id
	})

	t.Run("get conversation", func(t *testing.T) {
		req := &proto.GetAssistantConversationsRequest{
			Username: username,
		}
		conversations, err := identity.GetAssistantConversations(ctx, req)
		require.NoError(t, err)
		require.Len(t, conversations.Conversations, 1)
		require.Equal(t, conversationID, conversations.Conversations[0].Id)
	})

	t.Run("create message", func(t *testing.T) {
		msg := &proto.AssistantMessage{
			Username:       username,
			CreatedTime:    time.Now(),
			ConversationId: conversationID,
			Payload:        "foo",
			Type:           "USER_MSG",
		}
		err := identity.CreateAssistantMessage(ctx, msg)
		require.NoError(t, err)
	})

	t.Run("get messages", func(t *testing.T) {
		req := &proto.AssistantMessageRequest{
			Username:       username,
			ConversationId: conversationID,
		}
		messages, err := identity.GetAssistantMessages(ctx, req)
		require.NoError(t, err)
		require.Len(t, messages.Messages, 1)
		require.Equal(t, "foo", messages.Messages[0].Payload)
	})

	t.Run("set conversation title", func(t *testing.T) {
		titleReq := &proto.UpdateAssistantConversationInfoRequest{
			Title:          "bar",
			Username:       username,
			ConversationId: conversationID,
		}
		title := "bar"
		err := identity.UpdateAssistantConversationInfo(ctx, titleReq)
		require.NoError(t, err)

		req := &proto.GetAssistantConversationsRequest{
			Username: username,
		}

		conversations, err := identity.GetAssistantConversations(ctx, req)
		require.NoError(t, err)
		require.Len(t, conversations.Conversations, 1)
		require.Equal(t, title, conversations.Conversations[0].Title)
	})

	t.Run("conversations are sorted by created_time", func(t *testing.T) {
		req := &proto.CreateAssistantConversationRequest{
			Username:    username,
			CreatedTime: time.Now().Add(time.Hour),
		}

		conversationResp, err := identity.CreateAssistantConversation(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, conversationResp.Id)

		reqConversations := &proto.GetAssistantConversationsRequest{
			Username: username,
		}

		conversations, err := identity.GetAssistantConversations(ctx, reqConversations)
		require.NoError(t, err)
		require.Len(t, conversations.Conversations, 2)
		require.Equal(t, conversationID, conversations.Conversations[0].Id)
		require.Equal(t, conversationResp.Id, conversations.Conversations[1].Id)
	})
}
