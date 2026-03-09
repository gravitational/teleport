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
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type auditQueryCollection struct {
	auditQueries []*secreports.AuditQuery
}

func (c *auditQueryCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.auditQueries))
	for i, resource := range c.auditQueries {
		r[i] = resource
	}
	return r
}

func (c *auditQueryCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Title", "Query", "Description"})
	for _, v := range c.auditQueries {
		t.AddRow([]string{v.GetName(), v.Spec.Title, v.Spec.Query, v.Spec.Description})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func auditQueryHandler() Handler {
	return Handler{
		getHandler:    getAuditQuery,
		createHandler: createAuditQuery,
		deleteHandler: deleteAuditQuery,
		singleton:     false,
		mfaRequired:   true,
		description:   "A saved audit query, can be invoked directly or using Access Monitoring Rules. Requires Access Monitoring.",
	}
}

func getAuditQuery(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		auditQueries, err := client.SecReportsClient().GetSecurityAuditQueries(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &auditQueryCollection{auditQueries: auditQueries}, nil
	}

	auditQuery, err := client.SecReportsClient().GetSecurityAuditQuery(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &auditQueryCollection{auditQueries: []*secreports.AuditQuery{auditQuery}}, nil
}

func createAuditQuery(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	auditQuery, err := services.UnmarshalAuditQuery(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := auditQuery.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := client.SecReportsClient().UpsertSecurityAuditQuery(ctx, auditQuery); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("audit query %q upserted\n", auditQuery.GetName())
	return nil
}

func deleteAuditQuery(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	name := ref.Name
	if err := client.SecReportsClient().DeleteSecurityAuditQuery(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("audit query %q deleted\n", name)
	return nil
}
