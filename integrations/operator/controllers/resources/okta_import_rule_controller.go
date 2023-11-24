/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"context"

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
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
func NewOktaImportRuleReconciler(client kclient.Client, tClient *client.Client) *TeleportResourceReconciler[types.OktaImportRule, *resourcesv1.TeleportOktaImportRule] {
	oktaImportRuleClient := &oktaImportRuleClient{
		teleportClient: tClient,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.OktaImportRule, *resourcesv1.TeleportOktaImportRule](
		client,
		oktaImportRuleClient,
	)

	return resourceReconciler
}
