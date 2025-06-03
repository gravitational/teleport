package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createDiscoveryConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	discoveryConfig, err := services.UnmarshalDiscoveryConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	remote := client.DiscoveryConfigClient()

	if rc.force {
		if _, err := remote.UpsertDiscoveryConfig(ctx, discoveryConfig); err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf("DiscoveryConfig %q has been written\n", discoveryConfig.GetName())
		return nil
	}

	if _, err := remote.CreateDiscoveryConfig(ctx, discoveryConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("DiscoveryConfig %q has been created\n", discoveryConfig.GetName())

	return nil
}

func (rc *ResourceCommand) getDiscoveryConfig(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	remote := client.DiscoveryConfigClient()
	if rc.ref.Name != "" {
		dc, err := remote.GetDiscoveryConfig(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewDiscoveryConfigCollection([]*discoveryconfig.DiscoveryConfig{dc}), nil
	}

	var resources []*discoveryconfig.DiscoveryConfig
	var dcs []*discoveryconfig.DiscoveryConfig
	var err error
	var nextKey string
	for {
		dcs, nextKey, err = remote.ListDiscoveryConfigs(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, dcs...)
		if nextKey == "" {
			break
		}
	}

	return collections.NewDiscoveryConfigCollection(resources), nil
}
