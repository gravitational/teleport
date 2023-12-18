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

package local_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func newAssistService(t *testing.T) *local.AssistService {
	t.Helper()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	return local.NewAssistService(backend)
}

// TestAssistantCRUD tests the assistant CRUD operations.
func TestAssistantCRUD(t *testing.T) {
	t.Parallel()

	identity := newAssistService(t)
	ctx := context.Background()

	const username = "foo"
	var conversationID string

	t.Run("create conversation", func(t *testing.T) {
		req := &assist.CreateAssistantConversationRequest{
			Username:    username,
			CreatedTime: timestamppb.New(time.Now()),
		}

		conversationResp, err := identity.CreateAssistantConversation(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, conversationResp.Id)

		conversationID = conversationResp.Id
	})

	t.Run("get conversation", func(t *testing.T) {
		req := &assist.GetAssistantConversationsRequest{
			Username: username,
		}
		conversations, err := identity.GetAssistantConversations(ctx, req)
		require.NoError(t, err)
		require.Len(t, conversations.Conversations, 1)
		require.Equal(t, conversationID, conversations.Conversations[0].Id)
	})

	t.Run("create message", func(t *testing.T) {
		msg := &assist.CreateAssistantMessageRequest{
			Username:       username,
			ConversationId: conversationID,
			Message: &assist.AssistantMessage{
				CreatedTime: timestamppb.New(time.Now()),
				Payload:     "foo",
				Type:        "USER_MSG",
			},
		}
		err := identity.CreateAssistantMessage(ctx, msg)
		require.NoError(t, err)
	})

	t.Run("get messages", func(t *testing.T) {
		req := &assist.GetAssistantMessagesRequest{
			Username:       username,
			ConversationId: conversationID,
		}
		messages, err := identity.GetAssistantMessages(ctx, req)
		require.NoError(t, err)
		require.Len(t, messages.Messages, 1)
		require.Equal(t, "foo", messages.Messages[0].Payload)
	})

	t.Run("set conversation title", func(t *testing.T) {
		titleReq := &assist.UpdateAssistantConversationInfoRequest{
			Title:          "bar",
			Username:       username,
			ConversationId: conversationID,
		}
		title := "bar"
		err := identity.UpdateAssistantConversationInfo(ctx, titleReq)
		require.NoError(t, err)

		req := &assist.GetAssistantConversationsRequest{
			Username: username,
		}

		conversations, err := identity.GetAssistantConversations(ctx, req)
		require.NoError(t, err)
		require.Len(t, conversations.Conversations, 1)
		require.Equal(t, title, conversations.Conversations[0].Title)
	})

	t.Run("conversations are sorted by created_time", func(t *testing.T) {
		req := &assist.CreateAssistantConversationRequest{
			Username:    username,
			CreatedTime: timestamppb.New(time.Now().Add(time.Hour)),
		}

		conversationResp, err := identity.CreateAssistantConversation(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, conversationResp.Id)

		reqConversations := &assist.GetAssistantConversationsRequest{
			Username: username,
		}

		conversations, err := identity.GetAssistantConversations(ctx, reqConversations)
		require.NoError(t, err)
		require.Len(t, conversations.Conversations, 2)
		require.Equal(t, conversationID, conversations.Conversations[0].Id)
		require.Equal(t, conversationResp.Id, conversations.Conversations[1].Id)
	})

	t.Run("refuse to add messages if conversion does not exist", func(t *testing.T) {
		msg := &assist.CreateAssistantMessageRequest{
			Username:       username,
			ConversationId: uuid.New().String(),
			Message: &assist.AssistantMessage{
				CreatedTime: timestamppb.New(time.Now()),
				Payload:     "foo",
				Type:        "USER_MSG",
			},
		}
		err := identity.CreateAssistantMessage(ctx, msg)
		require.Error(t, err)
	})

	t.Run("delete conversation", func(t *testing.T) {
		req := &assist.DeleteAssistantConversationRequest{
			Username:       username,
			ConversationId: conversationID,
		}
		err := identity.DeleteAssistantConversation(ctx, req)
		require.NoError(t, err)

		reqConversations := &assist.GetAssistantConversationsRequest{
			Username: username,
		}

		conversations, err := identity.GetAssistantConversations(ctx, reqConversations)
		require.NoError(t, err)
		require.Len(t, conversations.Conversations, 1)
		require.NotEqual(t, conversationID, conversations.Conversations[0].Id, "conversation was not deleted")
	})
}
