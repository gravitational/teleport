// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
