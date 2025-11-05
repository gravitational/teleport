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
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type appCollection struct {
	apps []types.Application
}

// NewAppCollection creates a [Collection] over the provided applications.
func NewAppCollection(apps []types.Application) Collection {
	return &appCollection{apps: apps}
}

func (c *appCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.apps))
	for _, resource := range c.apps {
		r = append(r, resource)
	}
	return r
}

func (c *appCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, app := range c.apps {
		labels := common.FormatLabels(app.GetAllLabels(), verbose)
		rows = append(rows, []string{
			app.GetName(), app.GetDescription(), app.GetURI(), app.GetPublicAddr(), labels, app.GetVersion(),
		})
	}
	headers := []string{"Name", "Description", "URI", "Public Address", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func appHandler() Handler {
	return Handler{
		getHandler:    getApp,
		createHandler: createApp,
		updateHandler: updateApp,
		deleteHandler: deleteApp,
		singleton:     false,
		mfaRequired:   false,
		description:   "A dynamic resource representing an application that can be proxied via an application service.",
	}
}

func getApp(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		// TODO(tross): DELETE IN v21.0.0, replace with regular Collect
		apps, err := clientutils.CollectWithFallback(ctx, client.ListApps, client.GetApps)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return NewAppCollection(apps), nil
	}

	app, err := client.GetApp(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewAppCollection([]types.Application{app}), nil
}

func createApp(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	app, err := services.UnmarshalApp(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.CreateApp(ctx, app); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.Force {
				return trace.AlreadyExists("application %q already exists", app.GetName())
			}
			if err := client.UpdateApp(ctx, app); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("application %q has been updated\n", app.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	fmt.Printf("application %q has been created\n", app.GetName())
	return nil
}

func updateApp(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	app, err := services.UnmarshalApp(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.UpdateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("application %q has been updated\n", app.GetName())
	return nil
}

func deleteApp(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteApp(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("application %q has been deleted\n", ref.Name)
	return nil
}
