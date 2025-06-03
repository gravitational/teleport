package collections

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
	"io"
	"strings"
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
