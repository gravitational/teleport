package process

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

// initSAMLIdP initializes the SAML IdP.
//
//nolint:revive // Because we want this to be IdP.
func initSAMLIdP(ctx context.Context, cfg *servicecfg.Config, plugin *web.Plugin) error {
	log := cfg.Log.WithField(trace.Component, teleport.ComponentSAMLIdP)
	authClient := plugin.GetProxyClient()
	accessPoint := plugin.GetAccessPoint()

	// Create the authorizer.
	clusterName, err := authClient.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentSAMLIdP,
			Log:       log,
			Client:    authClient,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName.GetClusterName(),
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Get the public web address.
	publicAddr, err := cfg.Proxy.WebPublicAddr()
	if err != nil {
		return trace.Wrap(err)
	}

	// Initialize the SAML IdP.
	samlIdP, err := saml.New(ctx, saml.Config{
		Log:         log,
		Clock:       cfg.Clock,
		Client:      authClient,
		AccessPoint: accessPoint,
		Authorizer:  authorizer,
		BaseURL:     publicAddr,
		Emitter:     authClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	plugin.RegisterSAMLIdP(samlIdP)

	cfg.Log.Infof("SAML identity provider has started successfully.")

	return nil
}
