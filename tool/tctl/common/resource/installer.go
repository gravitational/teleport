package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getInstaller(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		installers, err := client.GetInstallers(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewInstallerCollection(installers), nil
	}
	inst, err := client.GetInstaller(ctx, rc.ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewInstallerCollection([]types.Installer{inst}), nil
}

func (rc *ResourceCommand) createInstaller(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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
