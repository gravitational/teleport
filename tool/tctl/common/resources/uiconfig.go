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
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type uiConfigCollection struct {
	uiconfig types.UIConfig
}

func (c *uiConfigCollection) Resources() []types.Resource {
	return []types.Resource{c.uiconfig}
}

func (c *uiConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Scrollback Lines", "Show Resources"})
	t.AddRow([]string{strconv.FormatInt(int64(c.uiconfig.GetScrollbackLines()), 10), string(c.uiconfig.GetShowResources())})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func uiConfigHandler() Handler {
	return Handler{
		getHandler:    getUIConfig,
		createHandler: createUIConfig,
		deleteHandler: deleteUIConfig,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures the Web UI settings.",
	}
}

func getUIConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindUIConfig)
	}
	uiconfig, err := client.GetUIConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &uiConfigCollection{uiconfig}, nil
}

func createUIConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	uic, err := services.UnmarshalUIConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.SetUIConfig(ctx, uic)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("ui_config %q has been set\n", uic.GetName())
	return nil
}

func deleteUIConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	err := client.DeleteUIConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s has been deleted\n", types.KindUIConfig)
	return nil
}
