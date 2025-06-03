package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createVnetConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	vnetConfig, err := services.UnmarshalProtoResource[*vnet.VnetConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rc.IsForced() {
		_, err = client.VnetConfigServiceClient().UpsertVnetConfig(ctx, &vnet.UpsertVnetConfigRequest{VnetConfig: vnetConfig})
	} else {
		_, err = client.VnetConfigServiceClient().CreateVnetConfig(ctx, &vnet.CreateVnetConfigRequest{VnetConfig: vnetConfig})
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("vnet_config has been created")
	return nil
}

func (rc *ResourceCommand) updateVnetConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	vnetConfig, err := services.UnmarshalProtoResource[*vnet.VnetConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.VnetConfigServiceClient().UpdateVnetConfig(ctx, &vnet.UpdateVnetConfigRequest{VnetConfig: vnetConfig}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("vnet_config has been updated")
	return nil
}

func (rc *ResourceCommand) getVnetConfig(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	vnetConfig, err := client.VnetConfigServiceClient().GetVnetConfig(ctx, &vnet.GetVnetConfigRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewVnetConfigCollection(vnetConfig), nil
}
