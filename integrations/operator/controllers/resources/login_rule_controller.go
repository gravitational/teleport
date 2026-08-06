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
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/entitlements"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/lib/modules"
)

// loginRuleClient implements TeleportResourceClient and offers CRUD methods needed to reconcile login_rules
type loginRuleClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport login_rule of a given name
func (l loginRuleClient) Get(ctx context.Context, key reconcilers.ResourceKey) (*resourcesv1.LoginRuleResource, error) {
	loginRule, err := l.teleportClient.GetLoginRule(ctx, key.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp := &resourcesv1.LoginRuleResource{LoginRule: loginRule}
	return resp, nil
}

// Create creates a Teleport login_rule
func (l loginRuleClient) Create(ctx context.Context, resource *resourcesv1.LoginRuleResource) error {
	_, err := l.teleportClient.CreateLoginRule(ctx, resource.LoginRule)
	return trace.Wrap(err)
}

// Update updates a Teleport login_rule
func (l loginRuleClient) Update(ctx context.Context, resource *resourcesv1.LoginRuleResource) error {
	_, err := l.teleportClient.UpsertLoginRule(ctx, resource.LoginRule)
	return trace.Wrap(err)
}

// Delete deletes a Teleport login_rule
func (l loginRuleClient) Delete(ctx context.Context, key reconcilers.ResourceKey) error {
	return trace.Wrap(l.teleportClient.DeleteLoginRule(ctx, key.Name))
}

// NewLoginRuleReconciler instantiates a new Kubernetes controller reconciling login_rule resources
func NewLoginRuleReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	loginRuleClient := &loginRuleClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithoutLabelsReconciler[*resourcesv1.LoginRuleResource, *resourcesv1.TeleportLoginRule](
		client,
		loginRuleClient,
		reconcilers.Config{
			CheckFeatures: func(features *proto.Features) bool {
				// Login Rules are enterprise-only but there is no specific feature flag for them.
				oidc := modules.GetProtoEntitlement(features, entitlements.OIDC)
				saml := modules.GetProtoEntitlement(features, entitlements.SAML)
				return oidc.Enabled || saml.Enabled
			},
		},
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
