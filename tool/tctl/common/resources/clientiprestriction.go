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
	"strings"

	"github.com/gravitational/trace"

	clientiprestrictionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func getClientIPRestriction(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if err := checkClientIPRestrictionName(ref); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := client.ClientIPRestrictionClient().GetClientIPRestriction(ctx, &clientiprestrictionv1.GetClientIPRestrictionRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clientIPRestrictionCollection{resp.GetClientIpRestriction()}, nil
}

// checkClientIPRestrictionName ensures a requested resource name, if any,
// matches the singleton's name. This prevents a typo'd or stale name from
// silently operating on the real allowlist (e.g. tctl edit
// client_ip_restriction/foo, which uses the get path before updating).
func checkClientIPRestrictionName(ref services.Ref) error {
	if ref.Name != "" && ref.Name != types.MetaNameClientIPRestriction {
		return trace.BadParameter("client_ip_restriction is a singleton, expected name %q, got %q", types.MetaNameClientIPRestriction, ref.Name)
	}
	return nil
}

func createClientIPRestriction(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cir, err := services.UnmarshalProtoResource[*clientiprestrictionv1.ClientIPRestriction](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		req := &clientiprestrictionv1.UpsertClientIPRestrictionRequest{}
		req.SetClientIpRestriction(cir)
		_, err = client.ClientIPRestrictionClient().UpsertClientIPRestriction(ctx, req)
	} else {
		req := &clientiprestrictionv1.CreateClientIPRestrictionRequest{}
		req.SetClientIpRestriction(cir)
		_, err = client.ClientIPRestrictionClient().CreateClientIPRestriction(ctx, req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("client_ip_restriction has been created")
	return nil
}

func updateClientIPRestriction(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cir, err := services.UnmarshalProtoResource[*clientiprestrictionv1.ClientIPRestriction](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	req := &clientiprestrictionv1.UpdateClientIPRestrictionRequest{}
	req.SetClientIpRestriction(cir)
	if _, err := client.ClientIPRestrictionClient().UpdateClientIPRestriction(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("client_ip_restriction has been updated")
	return nil
}

func deleteClientIPRestriction(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := checkClientIPRestrictionName(ref); err != nil {
		return trace.Wrap(err)
	}
	_, err := client.ClientIPRestrictionClient().DeleteClientIPRestriction(ctx, &clientiprestrictionv1.DeleteClientIPRestrictionRequest{})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("client_ip_restriction has been deleted")
	return nil
}

type clientIPRestrictionCollection struct {
	cir *clientiprestrictionv1.ClientIPRestriction
}

func (c *clientIPRestrictionCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.cir)}
}

func (c *clientIPRestrictionCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Allowed CIDRs", "State"})
	t.AddRow([]string{
		strings.Join(c.cir.GetSpec().GetAllowedCidrs(), ", "),
		c.cir.GetStatus().GetState(),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func clientIPRestrictionHandler() Handler {
	return Handler{
		getHandler:    getClientIPRestriction,
		createHandler: createClientIPRestriction,
		updateHandler: updateClientIPRestriction,
		deleteHandler: deleteClientIPRestriction,
		singleton:     true,
		mfaRequired:   false,
		description:   "Sets the IP ranges allowed to connect to a Cloud cluster.",
	}
}
