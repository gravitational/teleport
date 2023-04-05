package tbotv2

import (
	"context"
	"fmt"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/trace"
	"time"
)

type ApplicationDestination struct {
	AppName string
}

func (d *ApplicationDestination) Generate(ctx context.Context, bot BotI, store Store, roles []string, ttl time.Duration) error {
	id, err := bot.GenerateIdentity(ctx, IdentityRequest{
		roles: roles,
		ttl:   ttl,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	idClient, err := bot.ClientForIdentity(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}
	defer idClient.Close()

	routeToApp, err := d.getRouteToApp(ctx, id, idClient)
	if err != nil {
		return trace.Wrap(err)
	}

	routedIdentity, err := bot.GenerateIdentity(ctx, IdentityRequest{
		roles:      roles,
		routeToApp: routeToApp,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Persist to store
	err = store.Write(ctx, "tlscert", routedIdentity.TLSCertBytes)
	if err != nil {
		return trace.Wrap(err)
	}
	err = store.Write(ctx, "key", routedIdentity.PrivateKeyBytes)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func getApp(ctx context.Context, client auth.ClientI, appName string) (types.Application, error) {
	res, err := client.ListResources(ctx, proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindAppServer,
		PredicateExpression: fmt.Sprintf(`name == "%s"`, appName),
		Limit:               1,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(res.Resources).AsAppServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var apps []types.Application
	for _, server := range servers {
		apps = append(apps, server.GetApp())
	}
	apps = types.DeduplicateApps(apps)

	if len(apps) == 0 {
		return nil, trace.BadParameter("app %q not found", appName)
	}

	return apps[0], nil
}

func (d *ApplicationDestination) getRouteToApp(ctx context.Context, id *identity.Identity, client auth.ClientI) (proto.RouteToApp, error) {
	app, err := getApp(ctx, client, d.AppName)
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	// TODO: AWS?
	ws, err := client.CreateAppSession(ctx, types.CreateAppSessionRequest{
		ClusterName: id.ClusterName,
		Username:    id.X509Cert.Subject.CommonName,
		PublicAddr:  app.GetPublicAddr(),
	})
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	err = auth.WaitForAppSession(ctx, ws.GetName(), ws.GetUser(), client)
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	return proto.RouteToApp{
		Name:        app.GetName(),
		SessionID:   ws.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: id.ClusterName,
	}, nil
}
