package collections

import (
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
	"io"
	"strings"
)

type roleCollection struct {
	roles []types.Role
}

func NewRoleCollection(roles []types.Role) ResourceCollection {
	return &roleCollection{roles: roles}
}

func (r *roleCollection) Resources() (res []types.Resource) {
	for _, resource := range r.roles {
		res = append(res, resource)
	}
	return res
}

func (r *roleCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, r := range r.roles {
		if r.GetName() == constants.DefaultImplicitRole {
			continue
		}
		rows = append(rows, []string{
			r.GetMetadata().Name,
			strings.Join(r.GetLogins(types.Allow), ","),
			printNodeLabels(r.GetNodeLabels(types.Allow)),
			printActions(r.GetRules(types.Allow)),
		})
	}

	headers := []string{"Role", "Allowed to login as", "Node Labels", "Access to resources"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Access to resources")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
