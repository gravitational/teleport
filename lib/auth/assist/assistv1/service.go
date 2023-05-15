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

package assistv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for
// the assist gRPC service.
type ServiceConfig struct {
	Backend services.Assistant
}

// Service implements the teleport.assist.v1.AssistService RPC service.
type Service struct {
	assist.UnimplementedAssistServiceServer

	backend services.Assistant
}

// NewService returns a new assist gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	}

	return &Service{
		backend: cfg.Backend,
	}, nil
}

// CreateAssistantConversation creates a new conversation entry in the backend.
func (a *Service) CreateAssistantConversation(ctx context.Context, req *assist.CreateAssistantConversationRequest) (*assist.CreateAssistantConversationResponse, error) {
	resp, err := a.backend.CreateAssistantConversation(ctx, req)
	return resp, trace.Wrap(err)
}

// UpdateAssistantConversationInfo updates the conversation info for a conversation.
func (a *Service) UpdateAssistantConversationInfo(ctx context.Context, request *assist.UpdateAssistantConversationInfoRequest) (*emptypb.Empty, error) {
	err := a.backend.UpdateAssistantConversationInfo(ctx, request)
	if err != nil {
		return &emptypb.Empty{}, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// GetAssistantConversations returns all conversations started by a user.
func (a *Service) GetAssistantConversations(ctx context.Context, req *assist.GetAssistantConversationsRequest) (*assist.GetAssistantConversationsResponse, error) {
	resp, err := a.backend.GetAssistantConversations(ctx, req)
	return resp, trace.Wrap(err)
}

// GetAssistantMessages returns all messages with given conversation ID.
func (a *Service) GetAssistantMessages(ctx context.Context, req *assist.GetAssistantMessagesRequest) (*assist.GetAssistantMessagesResponse, error) {
	resp, err := a.backend.GetAssistantMessages(ctx, req)
	return resp, trace.Wrap(err)
}

// CreateAssistantMessage adds the message to the backend.
func (a *Service) CreateAssistantMessage(ctx context.Context, req *assist.CreateAssistantMessageRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, trace.Wrap(a.backend.CreateAssistantMessage(ctx, req))
}

// IsAssistEnabled returns true if the assist is enabled or not on the auth level.
func (a *Service) IsAssistEnabled(ctx context.Context, req *assist.IsAssistEnabledRequest) (*assist.IsAssistEnabledResponse, error) {
	return a.backend.IsAssistEnabled(ctx)
}
