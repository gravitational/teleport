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

func (rc *ResourceCommand) getAppServer(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	servers, err := client.GetApplicationServers(ctx, rc.namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewAppServerCollection(servers), nil
	}

	var out []types.AppServer
	for _, server := range servers {
		if server.GetName() == rc.ref.Name || server.GetHostname() == rc.ref.Name {
			out = append(out, server)
		}
	}
	if len(out) == 0 {
		return nil, trace.NotFound("application server %q not found", rc.ref.Name)
	}
	return collections.NewAppServerCollection(out), err
}

func (rc *ResourceCommand) createAppServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	appServer, err := services.UnmarshalAppServer(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if appServer.GetApp().GetIntegration() == "" {
		return trace.BadParameter("only applications that use an integration can be created")
	}
	if _, err := client.UpsertApplicationServer(ctx, appServer); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("application server %q has been upserted\n", appServer.GetName())
	return nil
}

func (rc *ResourceCommand) getApp(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		apps, err := client.GetApps(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewAppCollection(apps), nil
	}
	app, err := client.GetApp(ctx, rc.ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAppCollection([]types.Application{app}), nil
}

func (rc *ResourceCommand) createApp(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	app, err := services.UnmarshalApp(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.CreateApp(ctx, app); err != nil {
		if trace.IsAlreadyExists(err) {
			if !rc.force {
				return trace.AlreadyExists("application %q already exists", app.GetName())
			}
			if err := client.UpdateApp(ctx, app); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("application %q has been updated\n", app.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	fmt.Printf("application %q has been created\n", app.GetName())
	return nil
}
