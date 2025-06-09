package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var appServer = resource{
	getHandler:    getAppServer,
	createHandler: createAppServer,
	deleteHandler: deleteAppServer,
}

func getAppServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	servers, err := client.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewAppServerCollection(servers), nil
	}

	var out []types.AppServer
	for _, server := range servers {
		if server.GetName() == ref.Name || server.GetHostname() == ref.Name {
			out = append(out, server)
		}
	}
	if len(out) == 0 {
		return nil, trace.NotFound("application server %q not found", ref.Name)
	}
	return collections.NewAppServerCollection(out), err
}

func createAppServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteAppServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	appServers, err := client.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	deleted := false
	for _, server := range appServers {
		if server.GetName() == ref.Name {
			if err := client.DeleteApplicationServer(ctx, server.GetNamespace(), server.GetHostID(), server.GetName()); err != nil {
				return trace.Wrap(err)
			}
			deleted = true
		}
	}
	if !deleted {
		return trace.NotFound("application server %q not found", ref.Name)
	}
	fmt.Printf("application server %q has been deleted\n", ref.Name)
	return nil
}

var app = resource{
	getHandler:    getApp,
	createHandler: createApp,
	deleteHandler: deleteApp,
}

func getApp(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name == "" {
		apps, err := client.GetApps(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewAppCollection(apps), nil
	}
	app, err := client.GetApp(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAppCollection([]types.Application{app}), nil
}

func createApp(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	app, err := services.UnmarshalApp(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.CreateApp(ctx, app); err != nil {
		if trace.IsAlreadyExists(err) {
			if opts.force {
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

func deleteApp(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteApp(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("application %q has been deleted\n", ref.Name)
	return nil
}
