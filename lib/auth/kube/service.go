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
	kubev1.UnimplementedKubeServiceServer
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
		(*types.KubernetesClusterV3).GetName,
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
