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
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type oktaImportRuleCollection struct {
	importRules []types.OktaImportRule
}

func (c *oktaImportRuleCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.importRules))
	for i, resource := range c.importRules {
		r[i] = resource
	}
	return r
}

func (c *oktaImportRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, importRule := range c.importRules {
		t.AddRow([]string{importRule.GetName()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func oktaImportRuleHandler() Handler {
	return Handler{
		getHandler:    getOktaImportRule,
		createHandler: createOktaImportRule,
		deleteHandler: deleteOktaImportRule,
		description:   "Configures which Okta user groups and applications should be imported, and with which labels.",
	}
}

func getOktaImportRule(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		importRule, err := client.OktaClient().GetOktaImportRule(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oktaImportRuleCollection{importRules: []types.OktaImportRule{importRule}}, nil
	}

	importRules, err := stream.Collect(clientutils.Resources(ctx, client.OktaClient().ListOktaImportRules))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &oktaImportRuleCollection{importRules: importRules}, nil
}

func createOktaImportRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	importRule, err := services.UnmarshalOktaImportRule(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	exists := false
	if _, err = client.OktaClient().CreateOktaImportRule(ctx, importRule); err != nil {
		if trace.IsAlreadyExists(err) {
			exists = true
			_, err = client.OktaClient().UpdateOktaImportRule(ctx, importRule)
		}

		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("Okta import rule %q has been %s\n", importRule.GetName(), upsertVerb(exists, opts.Force))
	return nil
}

func deleteOktaImportRule(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.OktaClient().DeleteOktaImportRule(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Okta import rule %q has been deleted\n", ref.Name)
	return nil
}
