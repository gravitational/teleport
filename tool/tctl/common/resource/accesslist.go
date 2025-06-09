package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var accessList = resource{
	getHandler:    getAccessList,
	createHandler: createAccessList,
	deleteHandler: deleteAccessList,
	description:   "",
}

func createAccessList(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	accessList, err := services.UnmarshalAccessList(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = client.AccessListClient().GetAccessList(ctx, accessList.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)

	if exists && opts.force {
		return trace.AlreadyExists("Access list %q already exists", accessList.GetName())
	}

	if _, err := client.AccessListClient().UpsertAccessList(ctx, accessList); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Access list %q has been %s\n", accessList.GetName(), UpsertVerb(exists, opts.force))

	return nil
}

func getAccessList(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		resource, err := client.AccessListClient().GetAccessList(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewAccessListCollection([]*accesslist.AccessList{resource}), nil
	}
	accessLists, err := client.AccessListClient().GetAccessLists(ctx)

	return collections.NewAccessListCollection(accessLists), trace.Wrap(err)
}

func deleteAccessList(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.AccessListClient().DeleteAccessList(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Access list %q has been deleted\n", ref.Name)
	return nil
}
