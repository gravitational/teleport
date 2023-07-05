/*
Copyright 2023 Gravitational, Inc.

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

	resourcesv1 "github.com/gravitational/teleport/operator/apis/resources/v1"
	"github.com/gravitational/teleport/operator/sidecar"
)

// loginRuleClient implements TeleportResourceClient and offers CRUD methods needed to reconcile login_rules
type loginRuleClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

// Get gets the Teleport login_rule of a given name
func (l loginRuleClient) Get(ctx context.Context, name string) (*resourcesv1.LoginRuleResource, error) {
	teleportClient, err := l.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginRule, err := teleportClient.GetLoginRule(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp := &resourcesv1.LoginRuleResource{LoginRule: loginRule}
	return resp, nil
}

// Create creates a Teleport login_rule
func (l loginRuleClient) Create(ctx context.Context, resource *resourcesv1.LoginRuleResource) error {
	teleportClient, err := l.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = teleportClient.CreateLoginRule(ctx, resource.LoginRule)
	return trace.Wrap(err)
}

// Update updates a Teleport login_rule
func (l loginRuleClient) Update(ctx context.Context, resource *resourcesv1.LoginRuleResource) error {
	teleportClient, err := l.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = teleportClient.UpsertLoginRule(ctx, resource.LoginRule)
	return trace.Wrap(err)
}

// Delete deletes a Teleport login_rule
func (l loginRuleClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := l.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.DeleteLoginRule(ctx, name))
}

// NewLoginRuleReconciler instantiates a new Kubernetes controller reconciling login_rule resources
func NewLoginRuleReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[*resourcesv1.LoginRuleResource, *resourcesv1.TeleportLoginRule] {
	loginRuleClient := &loginRuleClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[*resourcesv1.LoginRuleResource, *resourcesv1.TeleportLoginRule](
		client,
		loginRuleClient,
	)

	return resourceReconciler
}
