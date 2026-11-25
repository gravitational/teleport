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
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewAppClient returns an app client.
func NewAppClient(c *client.Client) AppClient {
	return AppClient{client: c}
}

// AppClient manages app resources.
type AppClient struct {
	client *client.Client
}

// Get reads an app by name and scope.
func (r AppClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*types.AppV3, error) {
	app, err := r.client.GetApplication(ctx, presencev1.GetApplicationRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appv3, ok := app.(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("unexpected application type: %T", app)
	}

	return appv3, nil
}

// Create creates an app.
func (r AppClient) Create(ctx context.Context, app *types.AppV3) error {
	if err := r.client.CreateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates an app.
func (r AppClient) Upsert(ctx context.Context, app *types.AppV3) error {
	if err := r.client.UpdateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an app by name and scope.
func (r AppClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	return trace.Wrap(r.client.DeleteApplication(ctx, presencev1.DeleteApplicationRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build()))
}
