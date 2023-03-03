/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package process

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// initSAMLIdP initializes the SAML IdP.
//
//nolint:revive // Because we want this to be IdP.
func initSAMLIdP(ctx context.Context, cfg *service.Config, plugin *web.Plugin) error {
	log := cfg.Log.WithField(trace.Component, "samlidp")
	authClient := plugin.GetProxyClient()
	accessPoint := plugin.GetAccessPoint()

	// Create the authorizer.
	clusterName, err := authClient.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: saml.ComponentSAMLIdP,
			Log:       log,
			Client:    authClient,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authorizer, err := auth.NewAuthorizer(auth.AuthorizerOpts{
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

	if err := plugin.RegisterSAMLIdP(samlIdP); err != nil {
		return trace.Wrap(err)
	}

	cfg.Log.Infof("SAML identity provider has started successfully.")

	return nil
}
