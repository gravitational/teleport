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

package collections

import (
	"io"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/asciitable"
)

type auditQueryCollection struct {
	auditQueries []*secreports.AuditQuery
}

func NewAuditQueryCollection(auditQueries []*secreports.AuditQuery) ResourceCollection {
	return &auditQueryCollection{auditQueries: auditQueries}
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

type securityReportCollection struct {
	items []*secreports.Report
}

func NewSecurityReportCollection(reports []*secreports.Report) ResourceCollection {
	return &securityReportCollection{items: reports}
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
