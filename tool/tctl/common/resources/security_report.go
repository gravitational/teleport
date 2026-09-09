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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type securityReportCollection struct {
	items []*secreports.Report
}

func (c *securityReportCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.items))
	for i, resource := range c.items {
		r[i] = resource
	}
	return r
}

func (c *securityReportCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Title", "Audit Queries", "Description"})
	for _, v := range c.items {
		auditQueriesNames := make([]string, 0, len(v.Spec.AuditQueries))
		for _, k := range v.Spec.AuditQueries {
			auditQueriesNames = append(auditQueriesNames, k.Name)
		}
		t.AddRow([]string{v.GetName(), v.Spec.Title, strings.Join(auditQueriesNames, ", "), v.Spec.Description})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func securityReportHandler() Handler {
	return Handler{
		getHandler:    getSecurityReport,
		createHandler: createSecurityReport,
		deleteHandler: deleteSecurityReport,
		description:   "Defines a set of audit queries used to build a security report.",
	}
}

func getSecurityReport(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		resource, err := client.SecReportsClient().GetSecurityReport(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &securityReportCollection{items: []*secreports.Report{resource}}, nil
	}
	items, err := client.SecReportsClient().GetSecurityReports(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &securityReportCollection{items: items}, nil
}

func createSecurityReport(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	in, err := services.UnmarshalSecurityReport(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := in.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err = client.SecReportsClient().UpsertSecurityReport(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func deleteSecurityReport(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.SecReportsClient().DeleteSecurityReport(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Security report %q has been deleted\n", ref.Name)
	return nil
}
