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
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// oktaImportRuleClient implements TeleportResourceClient and offers CRUD methods needed to reconcile okta_import_rules
type oktaImportRuleClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport okta_import_rule of a given name
func (r oktaImportRuleClient) Get(ctx context.Context, name string) (types.OktaImportRule, error) {
	importRule, err := r.teleportClient.OktaClient().GetOktaImportRule(ctx, name)
	return importRule, trace.Wrap(err)
}

// Create creates a Teleport okta_import_rule
func (r oktaImportRuleClient) Create(ctx context.Context, importRule types.OktaImportRule) error {
	_, err := r.teleportClient.OktaClient().CreateOktaImportRule(ctx, importRule)
	return trace.Wrap(err)
}

// Update updates a Teleport okta_import_rule
func (r oktaImportRuleClient) Update(ctx context.Context, importRule types.OktaImportRule) error {
	_, err := r.teleportClient.OktaClient().UpdateOktaImportRule(ctx, importRule)
	return trace.Wrap(err)
}

// Delete deletes a Teleport okta_import_rule
func (r oktaImportRuleClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.OktaClient().DeleteOktaImportRule(ctx, name))
}

// NewOktaImportRuleReconciler instantiates a new Kubernetes controller reconciling okta_import_rule resources
func NewOktaImportRuleReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	oktaImportRuleClient := &oktaImportRuleClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.OktaImportRule, *resourcesv1.TeleportOktaImportRule](
		client,
		oktaImportRuleClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
