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

	// GetAssistantSettings returns the frontend settings for the assistant.
	GetAssistantSettings(ctx context.Context, req *assist.GetAssistantSettingsRequest) (*assist.AssistantSettings, error)

	// UpdateAssistantSettings updates the frontend settings for the assistant.
	UpdateAssistantSettings(ctx context.Context, req *assist.UpdateAssistantSettingsRequest) error
}
