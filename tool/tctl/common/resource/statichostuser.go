package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createStaticHostUser(ctx context.Context, client *authclient.Client, resource services.UnknownResource) error {
	hostUser, err := services.UnmarshalProtoResource[*userprovisioningpb.StaticHostUser](resource.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	c := client.StaticHostUserClient()
	if rc.force {
		if _, err := c.UpsertStaticHostUser(ctx, hostUser); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("static host user %q has been updated\n", hostUser.GetMetadata().Name)
	} else {
		if _, err := c.CreateStaticHostUser(ctx, hostUser); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("static host user %q has been created\n", hostUser.GetMetadata().Name)
	}

	return nil
}

func (rc *ResourceCommand) getStaticHostUser(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	hostUserClient := client.StaticHostUserClient()
	if rc.ref.Name != "" {
		hostUser, err := hostUserClient.GetStaticHostUser(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return collections.NewStaticHostUserCollection([]*userprovisioningpb.StaticHostUser{hostUser}), nil
	}

	var hostUsers []*userprovisioningpb.StaticHostUser
	var nextToken string
	for {
		resp, token, err := hostUserClient.ListStaticHostUsers(ctx, 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		hostUsers = append(hostUsers, resp...)
		if token == "" {
			break
		}
		nextToken = token
	}
	return collections.NewStaticHostUserCollection(hostUsers), nil
}

func (rc *ResourceCommand) updateStaticHostUser(ctx context.Context, client *authclient.Client, resource services.UnknownResource) error {
	hostUser, err := services.UnmarshalProtoResource[*userprovisioningpb.StaticHostUser](resource.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.StaticHostUserClient().UpdateStaticHostUser(ctx, hostUser); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("static host user %q has been updated\n", hostUser.GetMetadata().Name)
	return nil
}
