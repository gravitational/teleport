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

package provider

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
)

type appClient struct {
	client *client.Client
}

func (r appClient) Get(ctx context.Context, req GetResourceRequest[NameIdentifier]) (*types.AppV3, error) {
	app, err := r.client.GetApp(ctx, req.Identifier.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appv3, ok := app.(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("unexpected application type: %T", app)
	}

	return appv3, nil
}

func (r appClient) Create(ctx context.Context, req *types.AppV3) error {
	if err := r.client.CreateApp(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r appClient) Upsert(ctx context.Context, req *types.AppV3) error {
	if err := r.client.UpdateApp(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r appClient) Delete(ctx context.Context, req NameIdentifier) error {
	return trace.Wrap(r.client.DeleteApp(ctx, req.Name))
}
