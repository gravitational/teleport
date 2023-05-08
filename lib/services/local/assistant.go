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
 *
 */

package local

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/backend"
)

// Conversation is a conversation entry in the backend.
type Conversation struct {
	Title          string    `json:"title,omitempty"`
	ConversationID string    `json:"conversation_id"`
	CreatedTime    time.Time `json:"created_time"`
}

// CreateAssistantConversation creates a new conversation entry in the backend.
func (s *IdentityService) CreateAssistantConversation(ctx context.Context,
	req *proto.CreateAssistantConversationRequest,
) (*proto.CreateAssistantConversationResponse, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing parameter username")
	}

	conversationID := uuid.New().String()
	payload := &Conversation{
		ConversationID: conversationID,
		CreatedTime:    req.CreatedTime,
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.Key(assistantConversationPrefix, req.Username, conversationID),
		Value: value,
	}

	_, err = s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.CreateAssistantConversationResponse{Id: conversationID}, nil
}

func (s *IdentityService) getConversation(ctx context.Context, username, conversationID string) (*Conversation, error) {
	item, err := s.Get(ctx, backend.Key(assistantConversationPrefix, username, conversationID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var conversation Conversation
	if err := json.Unmarshal(item.Value, &conversation); err != nil {
		return nil, trace.Wrap(err)
	}

	return &conversation, nil
}

// UpdateAssistantConversationInfo updates the conversation title.
func (s *IdentityService) UpdateAssistantConversationInfo(ctx context.Context, request *proto.UpdateAssistantConversationInfoRequest) error {
	if request.ConversationId == "" {
		return trace.BadParameter("missing conversation ID")
	}
	if request.Username == "" {
		return trace.BadParameter("missing username")
	}
	if request.Title == "" {
		return trace.BadParameter("missing title")
	}

	msg, err := s.getConversation(ctx, request.Username, request.GetConversationId())
	if err != nil {
		return trace.Wrap(err)
	}

	// Only update the title, leave the rest of the fields intact.
	msg.Title = request.Title

	payload, err := json.Marshal(msg)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.Key(assistantConversationPrefix, request.GetUsername(), request.GetConversationId()),
		Value: payload,
	}

	if _, err = s.Update(ctx, item); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetAssistantConversations returns all conversations started by a user.
func (s *IdentityService) GetAssistantConversations(ctx context.Context, req *proto.GetAssistantConversationsRequest) (*proto.GetAssistantConversationsResponse, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	startKey := backend.Key(assistantConversationPrefix, req.Username)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conversationsIDs := make([]*proto.ConversationInfo, 0, len(result.Items))
	for _, item := range result.Items {
		payload := &Conversation{}
		if err := json.Unmarshal(item.Value, payload); err != nil {
			return nil, trace.Wrap(err)
		}

		conversationsIDs = append(conversationsIDs, &proto.ConversationInfo{
			Id:          payload.ConversationID,
			Title:       payload.Title,
			CreatedTime: payload.CreatedTime,
		})
	}

	sort.Slice(conversationsIDs, func(i, j int) bool {
		return conversationsIDs[i].CreatedTime.Before(conversationsIDs[j].GetCreatedTime())
	})

	return &proto.GetAssistantConversationsResponse{
		Conversations: conversationsIDs,
	}, nil
}

// GetAssistantMessages returns all messages with given conversation ID.
func (s *IdentityService) GetAssistantMessages(ctx context.Context, req *proto.AssistantMessageRequest) (*proto.GetAssistantMessagesResponse, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}

	if req.ConversationId == "" {
		return nil, trace.BadParameter("missing conversation ID")
	}

	startKey := backend.Key(assistantMessagePrefix, req.Username, req.ConversationId)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]*proto.AssistantMessage, len(result.Items))
	for i, item := range result.Items {
		var a proto.AssistantMessage
		if err := json.Unmarshal(item.Value, &a); err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = &a
	}

	sort.Slice(out, func(i, j int) bool {
		// Sort by created time.
		return out[i].CreatedTime.Before(out[j].GetCreatedTime())
	})

	return &proto.GetAssistantMessagesResponse{
		Messages: out,
	}, nil
}

// CreateAssistantMessage adds the message to the backend.
func (s *IdentityService) CreateAssistantMessage(ctx context.Context, msg *proto.AssistantMessage) error {
	if msg.Username == "" {
		return trace.BadParameter("missing username")
	}

	value, err := json.Marshal(msg)
	if err != nil {
		return trace.Wrap(err)
	}

	messageID := uuid.New().String()

	item := backend.Item{
		Key:   backend.Key(assistantMessagePrefix, msg.Username, msg.ConversationId, messageID),
		Value: value,
	}

	_, err = s.Create(ctx, item)
	return trace.Wrap(err)
}
