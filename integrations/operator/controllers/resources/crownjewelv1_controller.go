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
	crownjewelv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// crownJewelClient implements TeleportResourceClient and offers CRUD methods needed to reconcile crownJewel
type crownJewelClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport crownJewel of a given name
func (l crownJewelClient) Get(ctx context.Context, key reconcilers.ResourceKey) (*crownjewelv1pb.CrownJewel, error) {
	resp, err := l.teleportClient.CrownJewelServiceClient().
		GetCrownJewel(ctx, key.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Create creates a Teleport crownJewel
func (l crownJewelClient) Create(ctx context.Context, resource *crownjewelv1pb.CrownJewel) error {
	_, err := l.teleportClient.CrownJewelServiceClient().
		CreateCrownJewel(ctx, resource)
	return trace.Wrap(err)
}

// Update updates a Teleport crownJewel
func (l crownJewelClient) Update(ctx context.Context, resource *crownjewelv1pb.CrownJewel) error {
	_, err := l.teleportClient.CrownJewelServiceClient().
		UpsertCrownJewel(ctx, resource)
	return trace.Wrap(err)
}

// Delete deletes a Teleport crownJewel
func (l crownJewelClient) Delete(ctx context.Context, key reconcilers.ResourceKey) error {
	return trace.Wrap(l.teleportClient.CrownJewelServiceClient().DeleteCrownJewel(ctx, key.Name))
}

// NewCrownJewelV1Reconciler instantiates a new Kubernetes controller reconciling crownJewel
// resources
func NewCrownJewelV1Reconciler(client kclient.Client, tClient *client.Client, _ reconcilers.OperatorMetadata) (controllers.Reconciler, error) {
	crownJewelClient := &crownJewelClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*crownjewelv1pb.CrownJewel, *resourcesv1.TeleportCrownJewelV1,
	](
		client,
		crownJewelClient,
		reconcilers.Config{},
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
