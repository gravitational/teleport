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
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for
// the assist gRPC service.
type ServiceConfig struct {
	Backend        services.Assistant
	Embeddings     *ai.SimpleRetriever
	Embedder       ai.Embedder
	Authorizer     authz.Authorizer
	Logger         *logrus.Entry
	ResourceGetter ResourceGetter
}

// ResourceGetter represents a subset of the auth.Cache interface.
// Created to avoid circular dependencies.
type ResourceGetter interface {
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)
}

// Service implements the teleport.assist.v1.AssistService RPC service.
type Service struct {
	assist.UnimplementedAssistServiceServer
	assist.UnimplementedAssistEmbeddingServiceServer

	backend    services.Assistant
	embeddings *ai.SimpleRetriever
	// embedder is used to embed text into a vector.
	// It can be nil if the OpenAI API key is not set.
	embedder       ai.Embedder
	authorizer     authz.Authorizer
	log            *logrus.Entry
	resourceGetter ResourceGetter
}

// NewService returns a new assist gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Embeddings == nil:
		return nil, trace.BadParameter("embeddings is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.ResourceGetter == nil:
		return nil, trace.BadParameter("resource getter is required")
	case cfg.Logger == nil:
		cfg.Logger = logrus.WithField(trace.Component, "assist.service")
	}
	// Embedder can be nil is the OpenAI API key is not set.

	return &Service{
		backend:        cfg.Backend,
		embeddings:     cfg.Embeddings,
		embedder:       cfg.Embedder,
		authorizer:     cfg.Authorizer,
		resourceGetter: cfg.ResourceGetter,
		log:            cfg.Logger,
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

// DeleteAssistantConversation deletes a conversation entry and associated messages from the backend.
func (a *Service) DeleteAssistantConversation(ctx context.Context, req *assist.DeleteAssistantConversationRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, trace.Wrap(a.backend.DeleteAssistantConversation(ctx, req))
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
func (a *Service) IsAssistEnabled(ctx context.Context, _ *assist.IsAssistEnabledRequest) (*assist.IsAssistEnabledResponse, error) {
	if a.embedder == nil {
		// If the embedder is not configured, the assist is not enabled as we cannot compute embeddings.
		return &assist.IsAssistEnabledResponse{Enabled: false}, nil
	}

	// Check if assist can use the backend.
	return a.backend.IsAssistEnabled(ctx)
}

func (a *Service) GetAssistantEmbeddings(ctx context.Context, msg *assist.GetAssistantEmbeddingsRequest) (*assist.GetAssistantEmbeddingsResponse, error) {
	// TODO(jakule): The kind needs to be updated when we add more resources.
	authCtx, err := authz.AuthorizeWithVerbs(ctx, a.log, a.authorizer, true, types.KindNode, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if a.embedder == nil {
		return nil, trace.BadParameter("assist is not configured in auth server")
	}

	// Call the openAI API to get the embeddings for the query.
	embeddings, err := a.embedder.ComputeEmbeddings(ctx, []string{msg.ContentQuery})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(embeddings) == 0 {
		return nil, trace.NotFound("OpenAI embeddings returned no results")
	}

	// Use default values for the id and content, as we only care about the embeddings.
	queryEmbeddings := ai.NewEmbedding(msg.Kind, "", embeddings[0], [32]byte{})
	documents := a.embeddings.GetRelevant(queryEmbeddings, int(msg.Limit), func(id string, embedding *ai.Embedding) bool {
		// Run RBAC check on the embedded resource.
		node, err := a.resourceGetter.GetNode(ctx, "default", embedding.GetEmbeddedID())
		if err != nil {
			a.log.Tracef("failed to get node %q: %v", embedding.GetName(), err)
			return false
		}
		return authCtx.Checker.CheckAccess(node, services.AccessState{MFAVerified: true}) == nil
	})

	protoDocs := make([]*assist.EmbeddedDocument, 0, len(documents))
	for _, doc := range documents {
		node, err := a.resourceGetter.GetNode(ctx, "default", doc.GetEmbeddedID())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		content, err := ai.SerializeNode(node)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		protoDocs = append(protoDocs, &assist.EmbeddedDocument{
			Id:              doc.GetEmbeddedID(),
			Content:         string(content),
			SimilarityScore: float32(doc.SimilarityScore),
		})
	}

	return &assist.GetAssistantEmbeddingsResponse{
		Embeddings: protoDocs,
	}, nil
}
