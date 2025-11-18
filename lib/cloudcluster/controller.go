package cloudcluster

import (
	"context"
	"log/slog"
	"reflect"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	cloudclusterv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

const (
	defaultReconcilerPeriod = time.Minute
)

type Controller struct {
	log    *slog.Logger
	period time.Duration

	authServer *auth.Server
}

type cloudControllerID string

func NewController(authServer *auth.Server, log *slog.Logger, clock clockwork.Clock, period time.Duration, metricsRegistry *prometheus.Registry) (*Controller, error) {
	if period <= 0 {
		period = defaultReconcilerPeriod
	}

	c := Controller{
		log:        log,
		period:     period,
		authServer: authServer,
	}

	return &c, nil
}

func (c *Controller) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.period)
	defer ticker.Stop()

	var registeredCloudClusters map[cloudControllerID]*cloudclusterv1pb.CloudCluster

	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.C:
			var err error
			registeredCloudClusters, err = c.reconcile(ctx, registeredCloudClusters)
			if err != nil {
				c.log.Error("error reconciling", "err", err.Error())
			}
		}
	}

	return nil
}

func (c *Controller) reconcile(ctx context.Context, registeredCloudClusters map[cloudControllerID]*cloudclusterv1pb.CloudCluster) (map[cloudControllerID]*cloudclusterv1pb.CloudCluster, error) {
	allCloudClusters := make(map[cloudControllerID]*cloudclusterv1pb.CloudCluster)
	for cc, err := range clientutils.Resources(ctx, c.authServer.CloudClusterService.ListCloudClusters) {
		if err != nil {
			return registeredCloudClusters, trace.Wrap(err)
		}

		allCloudClusters[cloudControllerID(cc.GetMetadata().Name)] = cc
	}

	cfg := services.GenericReconcilerConfig[cloudControllerID, *cloudclusterv1pb.CloudCluster]{
		Matcher: func(*cloudclusterv1pb.CloudCluster) bool {
			return true
		},
		CompareResources: compareCloudClusters,
		GetCurrentResources: func() map[cloudControllerID]*cloudclusterv1pb.CloudCluster {
			return registeredCloudClusters
		},
		GetNewResources: func() map[cloudControllerID]*cloudclusterv1pb.CloudCluster {
			return allCloudClusters
		},
		OnCreate: c.createCloudCluster,
		OnUpdate: c.updateCloudCluster,
		OnDelete: c.deleteCloudCluster,
		Logger:   c.log.With("resource_type", types.KindCloudCluster),
	}

	r, err := services.NewGenericReconciler[cloudControllerID, *cloudclusterv1pb.CloudCluster](cfg)
	if err != nil {
		return registeredCloudClusters, trace.Wrap(err)
	}

	err = r.Reconcile(ctx)
	if err != nil {
		return registeredCloudClusters, trace.Wrap(err)
	}

	return allCloudClusters, nil
}

func (c *Controller) createCloudCluster(ctx context.Context, cc *cloudclusterv1pb.CloudCluster) error {
	c.log.InfoContext(ctx, "CloudCluster created", "name", cc.GetMetadata().Name)
	return nil
}

func (c *Controller) updateCloudCluster(ctx context.Context, ncc *cloudclusterv1pb.CloudCluster, occ *cloudclusterv1pb.CloudCluster) error {
	c.log.InfoContext(ctx, "CloudCluster updated", "name", ncc.GetMetadata().Name)
	return nil
}

func (c *Controller) deleteCloudCluster(ctx context.Context, cc *cloudclusterv1pb.CloudCluster) error {
	c.log.InfoContext(ctx, "CloudCluster deleted", "name", cc.GetMetadata().Name)
	return nil
}

func compareCloudClusters(a, b *cloudclusterv1pb.CloudCluster) int {
	return services.EqualFromBool(cloudClustersEqual(a, b))
}

func cloudClustersEqual(a, b *cloudclusterv1pb.CloudCluster) bool {
	// TOOD check status
	return metadataEqual(a.GetMetadata(), b.GetMetadata()) &&
		specEqual(a.GetSpec(), b.GetSpec())
}

func metadataEqual(a, b *headerv1.Metadata) bool {
	return a.GetName() == b.GetName() &&
		a.GetNamespace() == b.GetNamespace() &&
		a.GetDescription() == b.GetDescription() &&
		reflect.DeepEqual(a.GetLabels(), b.GetLabels())
}

func specEqual(a, b *cloudclusterv1pb.CloudClusterSpec) bool {
	return a.GetAuthRegion() == b.GetAuthRegion() &&
		a.GetBotName() == b.GetBotName() &&
		a.GetJoinMethod() == b.GetJoinMethod() &&
		allowsEqual(a.GetAllow(), b.GetAllow())
}

func allowsEqual(a, b []*cloudclusterv1pb.IAMAllow) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].GetAwsAccount() != b[i].GetAwsAccount() ||
			a[i].GetAwsArn() != b[i].GetAwsArn() {
			return false
		}
	}

	return true
}
