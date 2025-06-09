package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var installer = resource{
	getHandler:    getInstaller,
	createHandler: createInstaller,
	deleteHandler: deleteInstaller,
}

func getInstaller(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name == "" {
		installers, err := client.GetInstallers(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewInstallerCollection(installers), nil
	}
	inst, err := client.GetInstaller(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewInstallerCollection([]types.Installer{inst}), nil
}

func createInstaller(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	inst, err := services.UnmarshalInstaller(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.SetInstaller(ctx, inst)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("installer %q has been set\n", inst.GetName())
	return nil
}

func deleteInstaller(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	err := client.DeleteInstaller(ctx, ref.Name)
	if err != nil {
		return trace.Wrap(err)
	}
	if ref.Name == installers.InstallerScriptName {
		fmt.Printf("%s has been reset to a default value\n", ref.Name)
	} else {
		fmt.Printf("%s has been deleted\n", ref.Name)
	}
	return nil
}
