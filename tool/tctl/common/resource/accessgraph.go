package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	clusterconfigrec "github.com/gravitational/teleport/tool/tctl/common/clusterconfig"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getAccessGraphSettings(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	settings, err := client.ClusterConfigClient().GetAccessGraphSettings(ctx, &clusterconfigpb.GetAccessGraphSettingsRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rec, err := clusterconfigrec.ProtoToResource(settings)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAccessGraphSettingsCollection(rec), nil
}

func (rc *ResourceCommand) upsertAccessGraphSettings(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	settings, err := clusterconfigrec.UnmarshalAccessGraphSettings(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.ClusterConfigClient().UpsertAccessGraphSettings(ctx, &clusterconfigpb.UpsertAccessGraphSettingsRequest{AccessGraphSettings: settings}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("access_graph_settings has been upserted")
	return nil
}

func (rc *ResourceCommand) updateAccessGraphSettings(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	settings, err := clusterconfigrec.UnmarshalAccessGraphSettings(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.ClusterConfigClient().UpdateAccessGraphSettings(ctx, &clusterconfigpb.UpdateAccessGraphSettingsRequest{AccessGraphSettings: settings}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("access_graph_settings has been updated")
	return nil
}

func (rc *ResourceCommand) getCrownJewel(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	cjClient := client.CrownJewelsClient()
	var rules []*crownjewelv1.CrownJewel
	nextToken := ""
	for {
		resp, token, err := cjClient.ListCrownJewels(ctx, 0 /* default size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		rules = append(rules, resp...)

		if token == "" {
			break
		}
		nextToken = token
	}
	return collections.NewCrownJewelCollection(rules), nil
}

func (rc *ResourceCommand) createCrownJewel(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	crownJewel, err := services.UnmarshalCrownJewel(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.CrownJewelsClient()
	if rc.force {
		if _, err := c.UpsertCrownJewel(ctx, crownJewel); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown jewel %q has been updated\n", crownJewel.GetMetadata().GetName())
	} else {
		if _, err := c.CreateCrownJewel(ctx, crownJewel); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown jewel %q has been created\n", crownJewel.GetMetadata().GetName())
	}

	return nil
}

func (rc *ResourceCommand) updateCrownJewel(ctx context.Context, client *authclient.Client, resource services.UnknownResource) error {
	in, err := services.UnmarshalCrownJewel(resource.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.CrownJewelsClient().UpdateCrownJewel(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("crown jewel %q has been updated\n", in.GetMetadata().GetName())
	return nil
}
