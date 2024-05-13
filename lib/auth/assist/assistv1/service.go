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

package assistv1

import (
	"context"
	"slices"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
	embeddinglib "github.com/gravitational/teleport/lib/ai/embedding"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// maxSearchLimit is the maximum number of search results to return.
	// We have a hard cap due the simplistic design of our retriever which has quadratic complexity.
	maxSearchLimit = 100
)

// ServiceConfig holds configuration options for
// the assist gRPC service.
type ServiceConfig struct {
	Backend        services.Assistant
	Embeddings     *ai.SimpleRetriever
	Embedder       embeddinglib.Embedder
	Authorizer     authz.Authorizer
	Logger         *logrus.Entry
	ResourceGetter ResourceGetter
}

// ResourceGetter represents a subset of the auth.Cache interface.
// Created to avoid circular dependencies.
type ResourceGetter interface {
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)
	GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error)
	GetApp(ctx context.Context, name string) (types.Application, error)
	GetDatabase(ctx context.Context, name string) (types.Database, error)
	GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error)
}

// Service implements the teleport.assist.v1.AssistService RPC service.
type Service struct {
	assist.UnimplementedAssistServiceServer
	assist.UnimplementedAssistEmbeddingServiceServer

	backend    services.Assistant
	embeddings *ai.SimpleRetriever
	// embedder is used to embed text into a vector.
	// It can be nil if the OpenAI API key is not set.
	embedder       embeddinglib.Embedder
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
		cfg.Logger = logrus.WithField(teleport.ComponentKey, "assist.service")
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
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAssistant, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if userHasAccess(authCtx, req) {
		return nil, trace.AccessDenied("user %q is not allowed to create conversation for user %q", authCtx.User.GetName(), req.Username)
	}

	resp, err := a.backend.CreateAssistantConversation(ctx, req)
	return resp, trace.Wrap(err)
}

// UpdateAssistantConversationInfo updates the conversation info for a conversation.
func (a *Service) UpdateAssistantConversationInfo(ctx context.Context, req *assist.UpdateAssistantConversationInfoRequest) (*emptypb.Empty, error) {
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAssistant, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if userHasAccess(authCtx, req) {
		return nil, trace.AccessDenied("user %q is not allowed to update conversation for user %q", authCtx.User.GetName(), req.Username)
	}

	err = a.backend.UpdateAssistantConversationInfo(ctx, req)
	if err != nil {
		return &emptypb.Empty{}, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// GetAssistantConversations returns all conversations started by a user.
func (a *Service) GetAssistantConversations(ctx context.Context, req *assist.GetAssistantConversationsRequest) (*assist.GetAssistantConversationsResponse, error) {
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAssistant, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	if userHasAccess(authCtx, req) {
		return nil, trace.AccessDenied("user %q is not allowed to list conversations for user %q", authCtx.User.GetName(), req.GetUsername())
	}

	resp, err := a.backend.GetAssistantConversations(ctx, req)
	return resp, trace.Wrap(err)
}

// DeleteAssistantConversation deletes a conversation entry and associated messages from the backend.
func (a *Service) DeleteAssistantConversation(ctx context.Context, req *assist.DeleteAssistantConversationRequest) (*emptypb.Empty, error) {
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAssistant, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if userHasAccess(authCtx, req) {
		return nil, trace.AccessDenied("user %q is not allowed to delete conversation for user %q", authCtx.User.GetName(), req.GetUsername())
	}

	return &emptypb.Empty{}, trace.Wrap(a.backend.DeleteAssistantConversation(ctx, req))
}

// GetAssistantMessages returns all messages with given conversation ID.
func (a *Service) GetAssistantMessages(ctx context.Context, req *assist.GetAssistantMessagesRequest) (*assist.GetAssistantMessagesResponse, error) {
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAssistant, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if userHasAccess(authCtx, req) {
		return nil, trace.AccessDenied("user %q is not allowed to get messages for user %q", authCtx.User.GetName(), req.GetUsername())
	}

	resp, err := a.backend.GetAssistantMessages(ctx, req)
	return resp, trace.Wrap(err)
}

// CreateAssistantMessage adds the message to the backend.
func (a *Service) CreateAssistantMessage(ctx context.Context, req *assist.CreateAssistantMessageRequest) (*emptypb.Empty, error) {
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAssistant, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if userHasAccess(authCtx, req) {
		return nil, trace.AccessDenied("user %q is not allowed to create message for user %q", authCtx.User.GetName(), req.GetUsername())
	}

	return &emptypb.Empty{}, trace.Wrap(a.backend.CreateAssistantMessage(ctx, req))
}

// IsAssistEnabled returns true if the assist is enabled or not on the auth level.
func (a *Service) IsAssistEnabled(ctx context.Context, _ *assist.IsAssistEnabledRequest) (*assist.IsAssistEnabledResponse, error) {
	if !modules.GetModules().Features().Assist {
		// If the assist feature is not enabled on the license, the assist is not enabled.
		return &assist.IsAssistEnabledResponse{Enabled: false}, nil
	}

	// If the embedder is not configured, the assist is not enabled as we cannot compute embeddings.
	if a.embedder == nil {
		return &assist.IsAssistEnabledResponse{Enabled: false}, nil
	}

	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if this endpoint is called by a user or Proxy.
	if authz.IsLocalUser(*authCtx) {
		checkErr := authCtx.Checker.CheckAccessToRule(
			&services.Context{User: authCtx.User},
			defaults.Namespace, types.KindAssistant, types.VerbRead,
		)
		if checkErr != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// This endpoint is called from Proxy to check if the assist is enabled.
		// Proxy credentials are used instead of the user credentials.
		requestedByProxy := authz.HasBuiltinRole(*authCtx, string(types.RoleProxy))
		if !requestedByProxy {
			return nil, trace.AccessDenied("only proxy is allowed to call IsAssistEnabled endpoint")
		}
	}

	// Check if assist can use the backend.
	return a.backend.IsAssistEnabled(ctx)
}

func (a *Service) GetAssistantEmbeddings(ctx context.Context, msg *assist.GetAssistantEmbeddingsRequest) (*assist.GetAssistantEmbeddingsResponse, error) {
	switch msg.Kind {
	case types.KindNode, types.KindKubernetesCluster, types.KindApp, types.KindDatabase, types.KindWindowsDesktop:
	default:
		return nil, trace.BadParameter("resource kind %v is not supported", msg.Kind)
	}

	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(msg.Kind, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	if a.embedder == nil {
		return nil, trace.BadParameter("assist is not configured in auth server")
	}

	// Call the openAI API to get the embeddings for the query.
	embeddings, err := a.embedder.ComputeEmbeddings(ctx, []string{msg.Query})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(embeddings) == 0 {
		return nil, trace.NotFound("OpenAI embeddings returned no results")
	}

	// Use default values for the id and content, as we only care about the embeddings.
	queryEmbeddings := embeddinglib.NewEmbedding(msg.Kind, "", embeddings[0], [32]byte{})
	accessChecker := makeAccessChecker(ctx, a, authCtx, msg.Kind)
	documents := a.embeddings.GetRelevant(queryEmbeddings, int(msg.Limit), accessChecker)
	return assembleEmbeddingResponse(ctx, a, documents)
}

// SearchUnifiedResources returns a similarity-ordered list of resources from the unified resource cache
func (a *Service) SearchUnifiedResources(ctx context.Context, msg *assist.SearchUnifiedResourcesRequest) (*assist.SearchUnifiedResourcesResponse, error) {
	if a.embedder == nil {
		return nil, trace.BadParameter("assist is not configured in auth server")
	}

	// Call the openAI API to get the embeddings for the query.
	embeddings, err := a.embedder.ComputeEmbeddings(ctx, []string{msg.Query})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(embeddings) == 0 {
		return nil, trace.NotFound("OpenAI embeddings returned no results")
	}

	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use default values for the id and content, as we only care about the embeddings.
	queryEmbeddings := embeddinglib.NewEmbedding("", "", embeddings[0], [32]byte{})
	limit := max(msg.Limit, maxSearchLimit)
	accessChecker := makeAccessChecker(ctx, a, authCtx, msg.Kinds...)
	documents := a.embeddings.GetRelevant(queryEmbeddings, int(limit), accessChecker)
	return assembleSearchResponse(ctx, a, documents)
}

// userHasAccess returns true if the user should have access to the resource.
func userHasAccess(authCtx *authz.Context, req interface{ GetUsername() string }) bool {
	return !authz.IsCurrentUser(*authCtx, req.GetUsername()) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin))
}

func assembleSearchResponse(ctx context.Context, a *Service, documents []*ai.Document) (*assist.SearchUnifiedResourcesResponse, error) {
	resources := make([]types.ResourceWithLabels, 0, len(documents))

	for _, doc := range documents {
		var resource types.ResourceWithLabels
		var err error

		switch doc.EmbeddedKind {
		case types.KindNode:
			resource, err = a.resourceGetter.GetNode(ctx, defaults.Namespace, doc.GetEmbeddedID())
		case types.KindKubernetesCluster:
			resource, err = a.resourceGetter.GetKubernetesCluster(ctx, doc.GetEmbeddedID())
		case types.KindApp:
			resource, err = a.resourceGetter.GetApp(ctx, doc.GetEmbeddedID())
		case types.KindDatabase:
			resource, err = a.resourceGetter.GetDatabase(ctx, doc.GetEmbeddedID())
		case types.KindWindowsDesktop:
			desktops, err := a.resourceGetter.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{
				Name: doc.GetEmbeddedID(),
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			for _, d := range desktops {
				if d.GetName() == doc.GetEmbeddedID() {
					resource = d
					break
				}
			}

			if resource == nil {
				return nil, trace.NotFound("windows desktop %q not found", doc.GetEmbeddedID())
			}
		default:
			return nil, trace.BadParameter("resource kind %v is not supported", doc.EmbeddedKind)
		}

		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resource)
	}

	paginated, err := services.MakePaginatedResources(ctx, types.KindUnifiedResource, resources, nil /* requestable map */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &assist.SearchUnifiedResourcesResponse{
		Resources: paginated,
	}, nil
}

func assembleEmbeddingResponse(ctx context.Context, a *Service, documents []*ai.Document) (*assist.GetAssistantEmbeddingsResponse, error) {
	protoDocs := make([]*assist.EmbeddedDocument, 0, len(documents))

	for _, doc := range documents {
		var content []byte

		switch doc.EmbeddedKind {
		case types.KindNode:
			node, err := a.resourceGetter.GetNode(ctx, defaults.Namespace, doc.GetEmbeddedID())
			if err != nil {
				return nil, trace.Wrap(err)
			}

			content, err = embeddinglib.SerializeNode(node)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindKubernetesCluster:
			cluster, err := a.resourceGetter.GetKubernetesCluster(ctx, doc.GetEmbeddedID())
			if err != nil {
				return nil, trace.Wrap(err)
			}

			content, err = embeddinglib.SerializeKubeCluster(cluster)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindApp:
			app, err := a.resourceGetter.GetApp(ctx, doc.GetEmbeddedID())
			if err != nil {
				return nil, trace.Wrap(err)
			}

			content, err = embeddinglib.SerializeApp(app)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindDatabase:
			db, err := a.resourceGetter.GetDatabase(ctx, doc.GetEmbeddedID())
			if err != nil {
				return nil, trace.Wrap(err)
			}

			content, err = embeddinglib.SerializeDatabase(db)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case types.KindWindowsDesktop:
			desktops, err := a.resourceGetter.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{
				Name: doc.GetEmbeddedID(),
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			var desktop types.WindowsDesktop
			for _, d := range desktops {
				if d.GetName() == doc.GetEmbeddedID() {
					desktop = d
					break
				}
			}

			if desktop == nil {
				return nil, trace.NotFound("windows desktop %q not found", doc.GetEmbeddedID())
			}

			content, err = embeddinglib.SerializeWindowsDesktop(desktop)
			if err != nil {
				return nil, trace.Wrap(err)
			}
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

func makeAccessChecker(ctx context.Context, a *Service, authCtx *authz.Context, kinds ...string) func(id string, embedding *embeddinglib.Embedding) bool {
	return func(id string, embedding *embeddinglib.Embedding) bool {
		if !slices.Contains(kinds, embedding.EmbeddedKind) && len(kinds) > 0 {
			return false
		}

		var resource services.AccessCheckable
		var err error

		switch embedding.EmbeddedKind {
		case types.KindNode:
			resource, err = a.resourceGetter.GetNode(ctx, defaults.Namespace, embedding.GetEmbeddedID())
			if err != nil {
				a.log.Tracef("failed to get node %q: %v", embedding.GetName(), err)
				return false
			}
		case types.KindKubernetesCluster:
			resource, err = a.resourceGetter.GetKubernetesCluster(ctx, embedding.GetEmbeddedID())
			if err != nil {
				a.log.Tracef("failed to get kube cluster %q: %v", embedding.GetName(), err)
				return false
			}
		case types.KindApp:
			resource, err = a.resourceGetter.GetApp(ctx, embedding.GetEmbeddedID())
			if err != nil {
				a.log.Tracef("failed to get app %q: %v", embedding.GetName(), err)
				return false
			}
		case types.KindDatabase:
			resource, err = a.resourceGetter.GetDatabase(ctx, embedding.GetEmbeddedID())
			if err != nil {
				a.log.Tracef("failed to get database %q: %v", embedding.GetName(), err)
				return false
			}
		case types.KindWindowsDesktop:
			desktops, err := a.resourceGetter.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{
				Name: embedding.GetEmbeddedID(),
			})
			if err != nil {
				a.log.Tracef("failed to get windows desktop %q: %v", embedding.GetName(), err)
				return false
			}

			for _, d := range desktops {
				if d.GetName() == embedding.GetEmbeddedID() {
					resource = d
					break
				}
			}

			if resource == nil {
				a.log.Tracef("failed to find windows desktop %q: %v", embedding.GetName(), err)
				return false
			}
		default:
			a.log.Tracef("resource kind %v is not supported", embedding.EmbeddedKind)
			return false
		}

		return authCtx.Checker.CheckAccess(resource, services.AccessState{MFAVerified: true}) == nil
	}
}
