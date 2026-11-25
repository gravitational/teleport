// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	loginrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewLoginRuleClient returns a login rule client.
func NewLoginRuleClient(c *client.Client) LoginRuleClient {
	return LoginRuleClient{client: c}
}

// LoginRuleClient manages login rule resources.
type LoginRuleClient struct {
	client *client.Client
}

// Get reads a login rule by name.
func (r LoginRuleClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*loginrulev1.LoginRule, error) {
	loginRule, err := r.client.GetLoginRule(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return loginRule, nil
}

// Create creates a login rule.
func (r LoginRuleClient) Create(ctx context.Context, loginRule *loginrulev1.LoginRule) error {
	if _, err := r.client.UpsertLoginRule(ctx, loginRule); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a login rule.
func (r LoginRuleClient) Upsert(ctx context.Context, loginRule *loginrulev1.LoginRule) error {
	if _, err := r.client.UpsertLoginRule(ctx, loginRule); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a login rule by name.
func (r LoginRuleClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteLoginRule(ctx, id.Name))
}
