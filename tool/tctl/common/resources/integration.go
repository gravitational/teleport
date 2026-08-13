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
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

func NewIntegrationCollection(integrations []types.Integration) Collection {
	return &integrationCollection{integrations: integrations}
}

type integrationCollection struct {
	integrations []types.Integration
}

func (c *integrationCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.integrations))
	for _, ig := range c.integrations {
		r = append(r, ig)
	}
	return r
}

func (c *integrationCollection) WriteText(w io.Writer, verbose bool) error {
	sort.Sort(types.Integrations(c.integrations))
	var rows [][]string
	for _, ig := range c.integrations {
		var specProps []string
		switch ig.GetSubKind() {
		case types.IntegrationSubKindAWSOIDC:
			specProps = append(specProps, fmt.Sprintf("RoleARN=%s", ig.GetAWSOIDCIntegrationSpec().RoleARN))
		case types.IntegrationSubKindGitHub:
			specProps = append(specProps, fmt.Sprintf("Organization=%s", ig.GetGitHubIntegrationSpec().Organization))
		}
		rows = append(rows, []string{
			ig.GetName(), ig.GetSubKind(), strings.Join(specProps, ","),
		})
	}
	t := asciitable.MakeTable([]string{"Name", "Type", "Spec"}, rows...)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func integrationHandler() Handler {
	return Handler{
		getHandler:    getIntegration,
		createHandler: createIntegration,
		updateHandler: updateIntegration,
		deleteHandler: deleteIntegration,
		singleton:     false,
		// TODO(greedy52) add admin action MFA for integrations.
		mfaRequired: false,
		description: "An integration with an external service such as AWS, GitHub, or Azure.",
	}
}

func getIntegration(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		ig, err := client.GetIntegration(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewIntegrationCollection([]types.Integration{ig}), nil
	}

	resources, err := stream.Collect(clientutils.Resources(ctx, client.ListIntegrations))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewIntegrationCollection(resources), nil
}

func createIntegration(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	integration, err := services.UnmarshalIntegration(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	existingIntegration, err := client.GetIntegration(ctx, integration.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)

	if exists {
		if !opts.Force {
			return trace.AlreadyExists("Integration %q already exists", integration.GetName())
		}
		return trace.Wrap(updateExistingIntegration(ctx, client, existingIntegration, integration))
	}

	igV1, ok := integration.(*types.IntegrationV1)
	if !ok {
		return trace.BadParameter("unexpected Integration type %T", integration)
	}

	if _, err := client.CreateIntegration(ctx, igV1); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Integration %q has been created\n", integration.GetName())
	return nil
}

func updateIntegration(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	integration, err := services.UnmarshalIntegration(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	existingIntegration, err := client.GetIntegration(ctx, integration.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(updateExistingIntegration(ctx, client, existingIntegration, integration))
}

func updateExistingIntegration(ctx context.Context, client *authclient.Client, existingIntegration, integration types.Integration) error {
	if err := existingIntegration.CanChangeStateTo(integration); err != nil {
		return trace.Wrap(err)
	}

	switch integration.GetSubKind() {
	case types.IntegrationSubKindAWSOIDC:
		existingIntegration.SetAWSOIDCIntegrationSpec(integration.GetAWSOIDCIntegrationSpec())
	case types.IntegrationSubKindGitHub:
		existingIntegration.SetGitHubIntegrationSpec(integration.GetGitHubIntegrationSpec())
		if creds := integration.GetCredentials(); creds != nil {
			if err := existingIntegration.SetCredentials(creds); err != nil {
				return trace.Wrap(err)
			}
		}
	case types.IntegrationSubKindAWSRolesAnywhere:
		existingIntegration.SetAWSRolesAnywhereIntegrationSpec(integration.GetAWSRolesAnywhereIntegrationSpec())
	case types.IntegrationSubKindAzureOIDC:
		existingIntegration.SetAzureOIDCIntegrationSpec(integration.GetAzureOIDCIntegrationSpec())
	default:
		return trace.BadParameter("subkind %q is not supported", integration.GetSubKind())
	}

	if _, err := client.UpdateIntegration(ctx, existingIntegration); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Integration %q has been updated\n", integration.GetName())
	return nil
}

func deleteIntegration(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteIntegration(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Integration %q removed\n", ref.Name)
	return nil
}
