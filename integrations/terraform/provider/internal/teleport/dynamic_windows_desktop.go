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
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewDynamicWindowsDesktopClient returns a dynamic Windows desktop client.
func NewDynamicWindowsDesktopClient(c *client.Client) DynamicWindowsDesktopClient {
	return DynamicWindowsDesktopClient{client: c}
}

// DynamicWindowsDesktopClient manages dynamic Windows desktop resources.
type DynamicWindowsDesktopClient struct {
	client *client.Client
}

// Get reads a dynamic Windows desktop by name.
func (r DynamicWindowsDesktopClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.DynamicWindowsDesktopV1, error) {
	desktop, err := r.client.DynamicDesktopClient().GetDynamicWindowsDesktop(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	desktopv1, ok := desktop.(*types.DynamicWindowsDesktopV1)
	if !ok {
		return nil, trace.BadParameter("unexpected dynamic Windows desktop type: %T", desktop)
	}

	return desktopv1, nil
}

// Create creates a dynamic Windows desktop.
func (r DynamicWindowsDesktopClient) Create(ctx context.Context, desktop *types.DynamicWindowsDesktopV1) error {
	if _, err := r.client.DynamicDesktopClient().CreateDynamicWindowsDesktop(ctx, desktop); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a dynamic Windows desktop.
func (r DynamicWindowsDesktopClient) Upsert(ctx context.Context, desktop *types.DynamicWindowsDesktopV1) error {
	if _, err := r.client.DynamicDesktopClient().UpsertDynamicWindowsDesktop(ctx, desktop); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a dynamic Windows desktop by name.
func (r DynamicWindowsDesktopClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DynamicDesktopClient().DeleteDynamicWindowsDesktop(ctx, id.Name))
}
