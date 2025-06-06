package collections

import (
	"io"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/asciitable"
)

func NewAccessListCollection(lists []*accesslist.AccessList) ResourceCollection {
	return &accessListCollection{
		accessLists: lists,
	}
}

type accessListCollection struct {
	accessLists []*accesslist.AccessList
}

func (c *accessListCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.accessLists))
	for i, resource := range c.accessLists {
		r[i] = resource
	}
	return r
}

func (c *accessListCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Title", "Review Frequency", "Next Audit Date"})
	for _, al := range c.accessLists {
		t.AddRow([]string{
			al.GetName(),
			al.Spec.Title,
			al.Spec.Audit.Recurrence.Frequency.String(),
			al.Spec.Audit.NextAuditDate.Format(time.RFC822),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
