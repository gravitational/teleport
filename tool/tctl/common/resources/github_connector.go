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
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type githubConnectorCollection struct {
	connectors []types.GithubConnector
}

func (c *githubConnectorCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *githubConnectorCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Teams To Logins"})
	for _, conn := range c.connectors {
		t.AddRow([]string{conn.GetName(), formatTeamsToLogins(
			conn.GetTeamsToLogins())})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func formatTeamsToLogins(mappings []types.TeamMapping) string {
	var result []string
	for _, m := range mappings {
		result = append(result, fmt.Sprintf("@%v/%v: %v",
			m.Organization, m.Team, strings.Join(m.Logins, ", ")))
	}
	return strings.Join(result, ", ")
}

func githubConnectorHandler() Handler {
	return Handler{
		getHandler:    getGithubConnector,
		createHandler: createGithubConnector,
		updateHandler: updateGithubConnector,
		deleteHandler: deleteGithubConnector,
		singleton:     false,
		mfaRequired:   true,
		description:   "Configures how users can connect using GitHub.",
	}
}

func getGithubConnector(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		// TODO(okraport): DELETE IN v21.0.0, replace with regular collect.
		githubConnectors, err := clientutils.CollectWithFallback(
			ctx,
			func(ctx context.Context, limit int, start string) ([]types.GithubConnector, string, error) {
				return client.ListGithubConnectors(ctx, limit, start, opts.WithSecrets)
			},
			func(ctx context.Context) ([]types.GithubConnector, error) {
				return client.GetGithubConnectors(ctx, opts.WithSecrets)
			},
		)

		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &githubConnectorCollection{connectors: githubConnectors}, nil
	}
	connector, err := client.GetGithubConnector(ctx, ref.Name, opts.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &githubConnectorCollection{[]types.GithubConnector{connector}}, nil

}

func createGithubConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	connector, err := services.UnmarshalGithubConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		upserted, err := client.UpsertGithubConnector(ctx, connector)
		if err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf("authentication connector %q has been updated\n", upserted.GetName())
		return nil
	}

	created, err := client.CreateGithubConnector(ctx, connector)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("authentication connector %q already exists", connector.GetName())
		}
		return trace.Wrap(err)
	}

	fmt.Printf("authentication connector %q has been created\n", created.GetName())

	return nil
}

func updateGithubConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	connector, err := services.UnmarshalGithubConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateGithubConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been updated\n", connector.GetName())
	return nil
}

func deleteGithubConnector(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteGithubConnector(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("github connector %q has been deleted\n", ref.Name)
	return nil
}
