/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// autoUpdateConfigClient implements TeleportResourceClient and offers CRUD methods needed to reconcile autoUpdateConfig
type autoUpdateConfigClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport autoUpdateConfig of a given name
func (l autoUpdateConfigClient) Get(ctx context.Context, name string) (*autoupdatev1pb.AutoUpdateConfig, error) {
	resp, err := l.teleportClient.
		GetAutoUpdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Create creates a Teleport autoUpdateConfig
func (l autoUpdateConfigClient) Create(ctx context.Context, resource *autoupdatev1pb.AutoUpdateConfig) error {
	_, err := l.teleportClient.
		CreateAutoUpdateConfig(ctx, resource)
	return trace.Wrap(err)
}

// Update updates a Teleport autoUpdateConfig
func (l autoUpdateConfigClient) Update(ctx context.Context, resource *autoupdatev1pb.AutoUpdateConfig) error {
	_, err := l.teleportClient.
		UpsertAutoUpdateConfig(ctx, resource)
	return trace.Wrap(err)
}

// Delete deletes a Teleport autoUpdateConfig
func (l autoUpdateConfigClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(l.teleportClient.DeleteAutoUpdateConfig(ctx))
}

// NewAutoUpdateConfigV1Reconciler instantiates a new Kubernetes controller reconciling autoUpdateConfig
// resources
func NewAutoUpdateConfigV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	autoUpdateConfigClient := &autoUpdateConfigClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*autoupdatev1pb.AutoUpdateConfig, *resourcesv1.TeleportAutoupdateConfigV1,
	](
		client,
		autoUpdateConfigClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
