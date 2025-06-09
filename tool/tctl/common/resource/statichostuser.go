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

var staticHostUser = resource{
	getHandler:    getStaticHostUser,
	createHandler: createStaticHostUser,
	updateHandler: updateStaticHostUser,
	deleteHandler: deleteStaticHostUser,
}

func createStaticHostUser(ctx context.Context, client *authclient.Client, resource services.UnknownResource, opts createOpts) error {
	hostUser, err := services.UnmarshalProtoResource[*userprovisioningpb.StaticHostUser](resource.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	c := client.StaticHostUserClient()
	if opts.force {
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

func getStaticHostUser(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	hostUserClient := client.StaticHostUserClient()
	if ref.Name != "" {
		hostUser, err := hostUserClient.GetStaticHostUser(ctx, ref.Name)
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

func updateStaticHostUser(ctx context.Context, client *authclient.Client, resource services.UnknownResource, opts createOpts) error {
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

func deleteStaticHostUser(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.StaticHostUserClient().DeleteStaticHostUser(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("static host user %q has been deleted\n", ref.Name)
	return nil
}
