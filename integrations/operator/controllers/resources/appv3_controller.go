/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resources

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// appClient implements TeleportResourceClient and offers CRUD methods needed to reconcile apps
// Currently the same client is used by all app versions. If we need to treat
// them differently at some point, we can always split the client into separate clients.
type appClient struct {
	teleportClient     *client.Client
	watchAllNamespaces bool
}

func formatNamespacedAppName(name, namespace string) string {
	return fmt.Sprintf("%s.%s", name, namespace)
}

// appNameForContext returns the name to use for a Teleport app
// if watchAllNamespaces is true, it returns the name formatted as <app-name>.<k8s-namespace>, otherwise it returns the app name as is
func (r appClient) appNameForContext(ctx context.Context, name string) string {
	if !r.watchAllNamespaces {
		return name
	}

	crKey, ok := reconcilers.K8sCRKeyFromContext(ctx)
	if !ok || crKey.Namespace == "" {
		return name
	}

	return formatNamespacedAppName(name, crKey.Namespace)
}

// Get gets the Teleport app of a given name
func (r appClient) Get(ctx context.Context, name string) (types.Application, error) {
	appName := r.appNameForContext(ctx, name)
	app, err := r.teleportClient.GetApp(ctx, appName)

	// If app not found and we're using namespace suffixes, try the legacy unprefixed name
	// This handles migrations when watchAllNamespaces is enabled on existing deployments
	if trace.IsNotFound(err) && r.watchAllNamespaces && appName != name {
		app, err = r.teleportClient.GetApp(ctx, name)
	}

	return app, trace.Wrap(err)
}

// Create creates a Teleport app
func (r appClient) Create(ctx context.Context, app types.Application) error {
	app.SetName(r.appNameForContext(ctx, app.GetName()))
	return trace.Wrap(r.teleportClient.CreateApp(ctx, app))
}

// Update updates a Teleport app
func (r appClient) Update(ctx context.Context, app types.Application) error {
	app.SetName(r.appNameForContext(ctx, app.GetName()))
	return trace.Wrap(r.teleportClient.UpdateApp(ctx, app))
}

// Delete deletes a Teleport app
func (r appClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteApp(ctx, r.appNameForContext(ctx, name)))
}

// NewAppV3Reconciler instantiates a new Kubernetes controller reconciling app v6 resources
func NewAppV3Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	return NewAppV3ReconcilerWithNamespaceSuffix(client, tClient, false)
}

// NewAppV3ReconcilerWithNamespaceSuffix instantiates a new Kubernetes controller reconciling app v6 resources,
// optionally suffixing Teleport app names with the source Kubernetes namespace.
func NewAppV3ReconcilerWithNamespaceSuffix(client kclient.Client, tClient *client.Client, watchAllNamespaces bool) (controllers.Reconciler, error) {
	appClient := &appClient{
		teleportClient:     tClient,
		watchAllNamespaces: watchAllNamespaces,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.Application, *resourcesv1.TeleportAppV3](
		client,
		appClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
