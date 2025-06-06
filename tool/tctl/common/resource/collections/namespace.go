package collections

import (
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

type namespaceCollection struct {
	namespaces []types.Namespace
}

func NewNamespaceCollection(namespaces []types.Namespace) ResourceCollection {
	return &namespaceCollection{namespaces: namespaces}
}

func (n *namespaceCollection) Resources() (r []types.Resource) {
	for i := range n.namespaces {
		r = append(r, &n.namespaces[i])
	}
	return r
}

func (n *namespaceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, n := range n.namespaces {
		t.AddRow([]string{n.Metadata.Name})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
