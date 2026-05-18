package discovery

import (
	"cmp"
	"context"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/trace"
)

// correlateAzureDesktops builds an instance map from online Teleport Azure VM desktops.
func correlateAzureDesktops(desktops []types.WindowsDesktop) map[string]instanceInfo {
	instances := make(map[string]instanceInfo)
	for _, desktop := range desktops {
		labels := desktop.GetAllLabels()
		vmID := cmp.Or(labels[types.VMIDLabel], labels[types.VMIDLabelInternal])
		if vmID == "" {
			continue
		}
		info := instanceInfo{
			IsOnline: true,
			Region:   cmp.Or(labels[types.RegionLabel], labels[types.RegionLabelInternal]),
			Azure: &azureInfo{
				VMID:           vmID,
				SubscriptionID: cmp.Or(labels[types.SubscriptionIDLabel], labels[types.SubscriptionIDLabelInternal]),
				ResourceGroup:  cmp.Or(labels[types.ResourceGroupLabel], labels[types.ResourceGroupLabelInternal]),
			},
		}
		if !desktop.Expiry().IsZero() {
			info.Expiry = desktop.Expiry()
		}
		instances[vmID] = info
	}
	return instances
}

func buildWindowsDesktops(ctx context.Context, clt discoveryClient, from, to time.Time, cfg cloudProviderConfig) ([]instanceInfo, error) {
	_, azureEvents, err := getRunEvents(ctx, clt, from, to, cfg)
	if err != nil {
		return nil, trace.Wrap(err, "fetching installation audit events")
	}

	desktops, err := client.GetAllResources[types.WindowsDesktop](ctx, clt, &proto.ListResourcesRequest{
		ResourceType: types.KindWindowsDesktop,
		Namespace:    apidefaults.Namespace,
	})
	if err != nil {
		return nil, trace.Wrap(err, "fetching Windows Desktops")
	}

	tasks, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*usertasksv1.UserTask, string, error) {
		return clt.UserTasksClient().ListUserTasks(ctx, int64(limit), token, nil)
	}))
	if err != nil {
		return nil, trace.Wrap(err, "fetching user tasks")
	}

	var instances []instanceInfo
	if cfg.azure {
		azureInstances := cloudNodes(
			mergeInstances(correlateAzureRunEvents(azureEvents), correlateAzureDesktops(desktops)),
			tasks, azureTaskInstanceKeys)
		instances = append(instances, azureInstances...)
	}

	return instances, nil
}
