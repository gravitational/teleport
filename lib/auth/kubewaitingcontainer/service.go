// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubewaitingcontainer

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for
// the Kubernetes waiting container gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
	// Backend is the backend used to store Kubernetes waiting containers.
	Backend services.KubeWaitingContainer
	// Cache is the cache used to store Kubernetes waiting containers.
	Cache Cache
}

// Cache is responsible for getting Kubernetes
// ephemeral containers that are waiting to be created until moderated
// session conditions are met.
type Cache interface {
	ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error)
	GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error)
}

// Service implements the teleport.kubewaitingcontainer.v1.KubernetesWaitingContainer
// RPC service.
type Service struct {
	kubewaitingcontainerpb.UnimplementedKubeWaitingContainersServiceServer

	authorizer authz.Authorizer
	backend    services.KubeWaitingContainer
	cache      Cache
}

// NewService returns a new Kubernetes waiting container gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
	}, nil
}

// ListKubernetesWaitingContainers lists Kubernetes ephemeral
// containers that are waiting to be created until moderated
// session conditions are met.
func (s *Service) ListKubernetesWaitingContainers(ctx context.Context, req *kubewaitingcontainerpb.ListKubernetesWaitingContainersRequest) (*kubewaitingcontainerpb.ListKubernetesWaitingContainersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindKubeWaitingContainer, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	if !isKubeSvcOrProxy(authCtx) {
		return nil, trace.AccessDenied("unauthorized to list Kubernetes waiting container resources")
	}

	conts, nextToken, err := s.cache.ListKubernetesWaitingContainers(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &kubewaitingcontainerpb.ListKubernetesWaitingContainersResponse{
		WaitingContainers: conts,
		NextPageToken:     nextToken,
	}, nil
}

// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func (s *Service) GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	if req.Cluster == "" {
		return nil, trace.BadParameter("missing cluster")
	}
	if req.Namespace == "" {
		return nil, trace.BadParameter("missing namespace")
	}
	if req.PodName == "" {
		return nil, trace.BadParameter("missing pod name")
	}
	if req.ContainerName == "" {
		return nil, trace.BadParameter("missing container name")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindKubeWaitingContainer, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	if !isKubeSvcOrProxy(authCtx) {
		return nil, trace.AccessDenied("unauthorized to read Kubernetes waiting container resources")
	}

	out, err := s.cache.GetKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest{
		Username:      req.Username,
		Cluster:       req.Cluster,
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// CreateKubernetesWaitingContainer creates a Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func (s *Service) CreateKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindKubeWaitingContainer, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if !isKubeSvcOrProxy(authCtx) {
		return nil, trace.AccessDenied("unauthorized to create Kubernetes waiting container resources")
	}

	out, err := s.backend.CreateKubernetesWaitingContainer(ctx, req.WaitingContainer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// DeleteKubernetesWaitingContainer deletes a Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func (s *Service) DeleteKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest) (*emptypb.Empty, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	if req.Cluster == "" {
		return nil, trace.BadParameter("missing cluster")
	}
	if req.Namespace == "" {
		return nil, trace.BadParameter("missing namespace")
	}
	if req.PodName == "" {
		return nil, trace.BadParameter("missing pod name")
	}
	if req.ContainerName == "" {
		return nil, trace.BadParameter("missing container name")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindKubeWaitingContainer, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if !isKubeSvcOrProxy(authCtx) {
		return nil, trace.AccessDenied("unauthorized to delete Kubernetes waiting container resources")
	}

	return &emptypb.Empty{}, trace.Wrap(s.backend.DeleteKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest{
		Username:      req.Username,
		Cluster:       req.Cluster,
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
	}))
}

// isKubeSvcOrProxy returns true if the given context has the builtin role
// of "kube" or "proxy".
func isKubeSvcOrProxy(authCtx *authz.Context) bool {
	return authz.HasBuiltinRole(*authCtx, string(types.RoleKube)) || authz.HasBuiltinRole(*authCtx, string(types.RoleProxy))
}
