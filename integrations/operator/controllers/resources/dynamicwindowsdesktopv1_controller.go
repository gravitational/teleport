/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// dynamicWindowsDesktopClient implements TeleportResourceClient and offers
// CRUD methods needed to reconcile dynamic Windows desktops.
type dynamicWindowsDesktopClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport dynamic Windows desktop of a given name.
func (r dynamicWindowsDesktopClient) Get(ctx context.Context, key reconcilers.ResourceKey) (types.DynamicWindowsDesktop, error) {
	desktop, err := r.teleportClient.DynamicDesktopClient().GetDynamicWindowsDesktop(ctx, key.Name)
	return desktop, trace.Wrap(err)
}

// Create creates a Teleport dynamic Windows desktop.
func (r dynamicWindowsDesktopClient) Create(ctx context.Context, desktop types.DynamicWindowsDesktop) error {
	_, err := r.teleportClient.DynamicDesktopClient().CreateDynamicWindowsDesktop(ctx, desktop)
	return trace.Wrap(err)
}

// Update updates a Teleport dynamic Windows desktop.
func (r dynamicWindowsDesktopClient) Update(ctx context.Context, desktop types.DynamicWindowsDesktop) error {
	_, err := r.teleportClient.DynamicDesktopClient().UpdateDynamicWindowsDesktop(ctx, desktop)
	return trace.Wrap(err)
}

// Delete deletes a Teleport dynamic Windows desktop of a given name.
func (r dynamicWindowsDesktopClient) Delete(ctx context.Context, key reconcilers.ResourceKey) error {
	return trace.Wrap(r.teleportClient.DynamicDesktopClient().DeleteDynamicWindowsDesktop(ctx, key.Name))
}

// NewDynamicWindowsDesktopV1Reconciler instantiates a new Kubernetes controller reconciling
// dynamic Windows desktop v1 resources.
func NewDynamicWindowsDesktopV1Reconciler(client kclient.Client, tClient *client.Client, _ reconcilers.OperatorMetadata) (controllers.Reconciler, error) {
	dynamicWindowsDesktopClient := &dynamicWindowsDesktopClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.DynamicWindowsDesktop, *resourcesv1.TeleportDynamicWindowsDesktopV1](
		client,
		dynamicWindowsDesktopClient,
		reconcilers.Config{},
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
