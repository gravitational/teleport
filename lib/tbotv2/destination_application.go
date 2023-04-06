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
)

type ApplicationDestination struct {
	Common CommonDestination `yaml:",inline"`
	Name   string            `yaml:"name"`
}

func (d *ApplicationDestination) CheckAndSetDefaults() error {
	if d.Name == "" {
		return trace.BadParameter("application name must be specified")
	}
	return trace.Wrap(d.Common.CheckAndSetDefaults())
}

func (d *ApplicationDestination) Oneshot(ctx context.Context, bot BotI) error {
	return trace.Wrap(d.Generate(ctx, bot))
}

func (d *ApplicationDestination) Run(ctx context.Context, bot BotI) error {
	return trace.Wrap(d.Common.Run(ctx, bot, d.Generate))
}

func (d *ApplicationDestination) Generate(ctx context.Context, bot BotI) error {
	id, err := bot.GenerateIdentity(ctx, IdentityRequest{
		roles: d.Common.Roles,
		ttl:   d.Common.TTL,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	idClient, err := bot.ClientForIdentity(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}
	defer idClient.Close()

	routeToApp, err := getRouteToApp(ctx, id, idClient, d.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	routedIdentity, err := bot.GenerateIdentity(ctx, IdentityRequest{
		ttl:        d.Common.TTL,
		roles:      d.Common.Roles,
		routeToApp: routeToApp,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Persist to store
	err = d.Common.Store.Write(ctx, "tlscert", routedIdentity.TLSCertBytes)
	if err != nil {
		return trace.Wrap(err)
	}
	err = d.Common.Store.Write(ctx, "key", routedIdentity.PrivateKeyBytes)
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

func getRouteToApp(ctx context.Context, id *identity.Identity, client auth.ClientI, appName string) (proto.RouteToApp, error) {
	app, err := getApp(ctx, client, appName)
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
