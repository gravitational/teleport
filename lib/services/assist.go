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

package services

import (
	"context"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
)

type Assistant interface {
	// GetAssistantMessages returns all messages with given conversation ID.
	GetAssistantMessages(ctx context.Context, req *assist.GetAssistantMessagesRequest) (*assist.GetAssistantMessagesResponse, error)

	// CreateAssistantMessage adds the message to the backend.
	CreateAssistantMessage(ctx context.Context, msg *assist.CreateAssistantMessageRequest) error

	// CreateAssistantConversation creates a new conversation entry in the backend.
	CreateAssistantConversation(ctx context.Context, req *assist.CreateAssistantConversationRequest) (*assist.CreateAssistantConversationResponse, error)

	// DeleteAssistantConversation deletes a conversation entry and associated messages from the backend.
	DeleteAssistantConversation(ctx context.Context, req *assist.DeleteAssistantConversationRequest) error

	// GetAssistantConversations returns all conversations started by a user.
	GetAssistantConversations(ctx context.Context, request *assist.GetAssistantConversationsRequest) (*assist.GetAssistantConversationsResponse, error)

	// UpdateAssistantConversationInfo updates conversation info.
	UpdateAssistantConversationInfo(ctx context.Context, msg *assist.UpdateAssistantConversationInfoRequest) error

	// IsAssistEnabled returns true if the assist is enabled or not on the auth level.
	IsAssistEnabled(ctx context.Context) (*assist.IsAssistEnabledResponse, error)
}
