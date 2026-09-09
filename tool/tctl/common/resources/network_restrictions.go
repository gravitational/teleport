/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type netRestrictionsCollection struct {
	netRestricts types.NetworkRestrictions
}

func (c *netRestrictionsCollection) Resources() (r []types.Resource) {
	r = append(r, c.netRestricts)
	return
}

type netRestrictionsWriter struct {
	w   io.Writer
	err error
}

func (w *netRestrictionsWriter) write(s string) {
	if w.err == nil {
		_, w.err = w.w.Write([]byte(s))
	}
}

func (c *netRestrictionsCollection) writeList(as []types.AddressCondition, w *netRestrictionsWriter) {
	for _, a := range as {
		w.write(a.CIDR)
		w.write("\n")
	}
}

func (c *netRestrictionsCollection) WriteText(w io.Writer, verbose bool) error {
	out := &netRestrictionsWriter{w: w}
	out.write("ALLOW\n")
	c.writeList(c.netRestricts.GetAllow(), out)

	out.write("\nDENY\n")
	c.writeList(c.netRestricts.GetDeny(), out)
	return trace.Wrap(out.err)
}

func networkRestrictionsHandler() Handler {
	return Handler{
		getHandler:    getNetworkRestrictions,
		createHandler: createNetworkRestrictions,
		deleteHandler: deleteNetworkRestrictions,
		singleton:     true,
		description:   "Restricts the IP ranges and ports that SSH sessions started through Teleport can reach.",
	}
}

func getNetworkRestrictions(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	nr, err := client.GetNetworkRestrictions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &netRestrictionsCollection{nr}, nil
}

func createNetworkRestrictions(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	newNetRestricts, err := services.UnmarshalNetworkRestrictions(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.SetNetworkRestrictions(ctx, newNetRestricts); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("network restrictions have been updated\n")
	return nil
}

func deleteNetworkRestrictions(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteNetworkRestrictions(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("network restrictions have been reset to defaults (allow all)\n")
	return nil
}
