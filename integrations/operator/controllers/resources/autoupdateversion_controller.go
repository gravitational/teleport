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

// autoUpdateVersionClient implements TeleportResourceClient and offers CRUD methods needed to reconcile autoUpdateVersion
type autoUpdateVersionClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport autoUpdateVersion of a given name
func (l autoUpdateVersionClient) Get(ctx context.Context, name string) (*autoupdatev1pb.AutoUpdateVersion, error) {
	resp, err := l.teleportClient.
		GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Create creates a Teleport autoUpdateVersion
func (l autoUpdateVersionClient) Create(ctx context.Context, resource *autoupdatev1pb.AutoUpdateVersion) error {
	_, err := l.teleportClient.
		CreateAutoUpdateVersion(ctx, resource)
	return trace.Wrap(err)
}

// Update updates a Teleport autoUpdateVersion
func (l autoUpdateVersionClient) Update(ctx context.Context, resource *autoupdatev1pb.AutoUpdateVersion) error {
	_, err := l.teleportClient.
		UpsertAutoUpdateVersion(ctx, resource)
	return trace.Wrap(err)
}

// Delete deletes a Teleport autoUpdateVersion
func (l autoUpdateVersionClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(l.teleportClient.DeleteAutoUpdateVersion(ctx))
}

// NewAutoUpdateVersionV1Reconciler instantiates a new Kubernetes controller reconciling autoUpdateVersion
// resources
func NewAutoUpdateVersionV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	autoUpdateVersionClient := &autoUpdateVersionClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*autoupdatev1pb.AutoUpdateVersion, *resourcesv1.TeleportAutoupdateVersionV1,
	](
		client,
		autoUpdateVersionClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
