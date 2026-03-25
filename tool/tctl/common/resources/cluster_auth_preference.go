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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type authPreferenceCollection struct {
	authPref types.AuthPreference
}

func (c *authPreferenceCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.authPref}
}

func (c *authPreferenceCollection) WriteText(w io.Writer, verbose bool) error {
	var secondFactorStrings []string
	for _, sf := range c.authPref.GetSecondFactors() {
		sfString, err := sf.Encode()
		if err != nil {
			return trace.Wrap(err)
		}
		secondFactorStrings = append(secondFactorStrings, sfString)
	}

	t := asciitable.MakeTable([]string{"Type", "Second Factors"})
	t.AddRow([]string{c.authPref.GetType(), strings.Join(secondFactorStrings, ", ")})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func authPreferenceHandler() Handler {
	return Handler{
		getHandler:    getAuthPreference,
		createHandler: createAuthPreference,
		updateHandler: updateAuthPreference,
		deleteHandler: deleteAuthPreference,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures the cluster authentication. Can only be used when auth is not configured in teleport.yaml.",
	}
}

func getAuthPreference(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindClusterAuthPreference)
	}
	authPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &authPreferenceCollection{authPref}, nil
}

func createAuthPreference(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	newAuthPref, err := services.UnmarshalAuthPreference(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedAuthPref, "cluster auth preference", opts.Force, opts.Confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertAuthPreference(ctx, newAuthPref); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster auth preference has been created\n")
	return nil
}

func updateAuthPreference(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	newAuthPref, err := services.UnmarshalAuthPreference(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedAuthPref, "cluster auth preference", opts.Confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateAuthPreference(ctx, newAuthPref); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster auth preference has been updated\n")
	return nil
}

func deleteAuthPreference(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedAuthPref.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	if err := client.ResetAuthPreference(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster auth preference has been reset to defaults\n")
	return nil
}
