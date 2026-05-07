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
	accessmonitoringrulesv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// accessMonitoringRuleClient implements TeleportResourceClient and offers CRUD methods needed to reconcile accessMonitoringRule
type accessMonitoringRuleClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport accessMonitoringRule of a given name
func (l accessMonitoringRuleClient) Get(ctx context.Context, name string) (*accessmonitoringrulesv1pb.AccessMonitoringRule, error) {
	resp, err := l.teleportClient.AccessMonitoringRulesClient().
		GetAccessMonitoringRule(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Create creates a Teleport accessMonitoringRule
func (l accessMonitoringRuleClient) Create(ctx context.Context, resource *accessmonitoringrulesv1pb.AccessMonitoringRule) error {
	_, err := l.teleportClient.AccessMonitoringRulesClient().
		CreateAccessMonitoringRule(ctx, resource)
	return trace.Wrap(err)
}

// Update updates a Teleport accessMonitoringRule
func (l accessMonitoringRuleClient) Update(ctx context.Context, resource *accessmonitoringrulesv1pb.AccessMonitoringRule) error {
	_, err := l.teleportClient.AccessMonitoringRulesClient().
		UpsertAccessMonitoringRule(ctx, resource)
	return trace.Wrap(err)
}

// Delete deletes a Teleport accessMonitoringRule
func (l accessMonitoringRuleClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(l.teleportClient.AccessMonitoringRulesClient().DeleteAccessMonitoringRule(ctx, name))
}

// NewAccessMonitoringRuleV1Reconciler instantiates a new Kubernetes controller reconciling accessMonitoringRule
// resources
func NewAccessMonitoringRuleV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	accessMonitoringRuleClient := &accessMonitoringRuleClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*accessmonitoringrulesv1pb.AccessMonitoringRule, *resourcesv1.TeleportAccessMonitoringRuleV1,
	](
		client,
		accessMonitoringRuleClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
