// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package kube

import (
	"context"
	"iter"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"

	kubev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

type ClusterReader interface {
	GetKubeCluster(ctx context.Context, req *kubev1.GetKubeClusterRequest) (types.KubeCluster, error)
	RangeKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest, startKey, endKey string) iter.Seq2[types.KubeCluster, error]
}

type ClusterWriter interface {
	DeleteKubeCluster(ctx context.Context, req *kubev1.DeleteKubeClusterRequest) error
}

type Config struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Logger           *slog.Logger
	Emitter          apievents.Emitter
	ClusterReader    ClusterReader
	ClusterWriter    ClusterWriter
}

type Service struct {
	cfg *Config
	kubev1.UnimplementedKubeClusterServiceServer
}

func NewService(cfg *Config) (*Service, error) {
	switch {
	case cfg.ScopedAuthorizer == nil:
		return nil, trace.BadParameter("ScopedAuthorizer must be provided")
	case cfg.ClusterReader == nil:
		return nil, trace.BadParameter("ClusterReader must be provided")
	case cfg.ClusterWriter == nil:
		return nil, trace.BadParameter("ClusterWriter must be provided")
	case cfg.Logger == nil:
		cfg.Logger = slog.With(teleport.ComponentKey, "kube")
	}
	return &Service{
		cfg: cfg,
	}, nil
}

func (s *Service) GetKubeCluster(ctx context.Context, req *kubev1.GetKubeClusterRequest) (*kubev1.GetKubeClusterResponse, error) {
	authContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindKubernetesCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := s.cfg.ClusterReader.GetKubeCluster(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authContext.CheckerContext.Decision(ctx, cluster.GetScope(), func(checker *services.ScopedAccessChecker) error {
		if err := checker.CheckAccessToRules(&ruleCtx, types.KindKubernetesCluster, types.VerbRead); err != nil {
			return err
		}
		return checker.Kube().CanAccessCluster(cluster)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterV3, ok := cluster.(*types.KubernetesClusterV3)
	if !ok {
		return nil, trace.BadParameter("invalid cluster")
	}
	return kubev1.GetKubeClusterResponse_builder{
		Cluster: clusterV3,
	}.Build(), nil
}

// getCursorForKubeCluster wraps [services.GetCursorForKubeCluster] with a signature
// referencing [*types.KubernetesClusterV3] directly. This helps go infer the proper
// typing when using [generic.CollectPageAndCursor].
func getCursorForKubeCluster(cluster *types.KubernetesClusterV3) string {
	return services.GetCursorForKubeCluster(cluster)
}

func (s *Service) ListKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest) (*kubev1.ListKubeClustersResponse, error) {
	authContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindKubernetesCluster, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	clusters, nextToken, err := generic.CollectPageAndCursor(
		stream.FilterMap(
			s.cfg.ClusterReader.RangeKubeClusters(ctx, req, req.GetPageToken(), ""),
			func(cluster types.KubeCluster) (*types.KubernetesClusterV3, bool) {
				// Filter out kube clusters user doesn't have access to.
				if err := authContext.CheckerContext.Decision(ctx, cluster.GetScope(), func(checker *services.ScopedAccessChecker) error {
					if err := checker.CheckAccessToRules(&ruleCtx, types.KindKubernetesCluster, types.VerbRead, types.VerbList); err != nil {
						return err
					}
					return checker.Kube().CanAccessCluster(cluster)
				}); err == nil {
					clusterV3, ok := cluster.(*types.KubernetesClusterV3)
					return clusterV3, ok
				}
				return nil, false
			},
		),
		int(req.GetPageSize()),
		getCursorForKubeCluster,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return kubev1.ListKubeClustersResponse_builder{
		Clusters:      clusters,
		NextPageToken: nextToken,
	}.Build(), nil
}

func (s *Service) DeleteKubeCluster(ctx context.Context, req *kubev1.DeleteKubeClusterRequest) (*kubev1.DeleteKubeClusterResponse, error) {
	authContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authContext.RuleContext()
	if err := authContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, types.KindKubernetesCluster, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// Make sure user has access to the kubernetes cluster before deleting.
	cluster, err := s.cfg.ClusterReader.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authContext.CheckerContext.Decision(ctx, cluster.GetScope(), func(checker *services.ScopedAccessChecker) error {
		if err := checker.CheckAccessToRules(&ruleCtx, types.KindKubernetesCluster, types.VerbDelete); err != nil {
			return err
		}
		return checker.Kube().CanAccessCluster(cluster)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.cfg.ClusterWriter.DeleteKubeCluster(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return kubev1.DeleteKubeClusterResponse_builder{}.Build(), nil
}
