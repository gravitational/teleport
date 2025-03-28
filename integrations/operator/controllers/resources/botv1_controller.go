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
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// botClient implements TeleportResourceClient and offers CRUD methods needed to reconcile bot
type botClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport bot of a given name
func (l botClient) Get(ctx context.Context, name string) (*machineidv1.Bot, error) {
	resp, err := l.teleportClient.
		BotServiceClient().
		GetBot(ctx, &machineidv1.GetBotRequest{BotName: name})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Create creates a Teleport bot
func (l botClient) Create(ctx context.Context, resource *machineidv1.Bot) error {
	_, err := l.teleportClient.
		BotServiceClient().
		CreateBot(ctx, &machineidv1.CreateBotRequest{Bot: resource})
	return trace.Wrap(err)
}

// Update updates a Teleport bot
func (l botClient) Update(ctx context.Context, resource *machineidv1.Bot) error {
	_, err := l.teleportClient.
		BotServiceClient().
		UpsertBot(ctx, &machineidv1.UpsertBotRequest{Bot: resource})
	return trace.Wrap(err)
}

// Delete deletes a Teleport bot
func (l botClient) Delete(ctx context.Context, name string) error {
	_, err := l.teleportClient.
		BotServiceClient().
		DeleteBot(ctx, &machineidv1.DeleteBotRequest{BotName: name})
	return trace.Wrap(err)
}

// NewBotV1Reconciler instantiates a new Kubernetes controller reconciling bot
// resources
func NewBotV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	botClient := &botClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*machineidv1.Bot, *resourcesv1.TeleportBotV1,
	](
		client,
		botClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
