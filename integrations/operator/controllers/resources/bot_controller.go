/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// botClient implements TeleportResourceClient and offers CRUD methods needed to reconcile bot
type botClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport bot of a given name
func (r botClient) Get(ctx context.Context, name string) (*machineidv1pb.Bot, error) {
	bot, err := r.teleportClient.BotServiceClient().GetBot(ctx, &machineidv1pb.GetBotRequest{
		BotName: name,
	})
	return bot, trace.Wrap(err)
}

// Create creates a Teleport bot
func (r botClient) Create(ctx context.Context, bot *machineidv1pb.Bot) error {
	_, err := r.teleportClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: bot,
	})
	return trace.Wrap(err)
}

// Update updates a Teleport bot
func (r botClient) Update(ctx context.Context, bot *machineidv1pb.Bot) error {
	_, err := r.teleportClient.BotServiceClient().UpdateBot(ctx, &machineidv1pb.UpdateBotRequest{
		Bot:        bot,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"spec.roles", "spec.traits"}},
	})
	return trace.Wrap(err)
}

// Delete deletes a Teleport bot
func (r botClient) Delete(ctx context.Context, name string) error {
	_, err := r.teleportClient.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{
		BotName: name,
	})
	return trace.Wrap(err)
}

// NewBotReconciler instantiates a new Kubernetes controller reconciling bot resources
func NewBotReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	botClient := &botClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[*machineidv1pb.Bot, *resourcesv1.TeleportBot](
		client,
		botClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
