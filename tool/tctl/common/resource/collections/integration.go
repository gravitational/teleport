package collections

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

type integrationCollection struct {
	integrations []types.Integration
}

func NewIntegrationCollection(integrations []types.Integration) ResourceCollection {
	return &integrationCollection{integrations: integrations}
}

func (c *integrationCollection) Resources() (r []types.Resource) {
	for _, ig := range c.integrations {
		r = append(r, ig)
	}
	return r
}

func (c *integrationCollection) WriteText(w io.Writer, verbose bool) error {
	sort.Sort(types.Integrations(c.integrations))
	var rows [][]string
	for _, ig := range c.integrations {
		specProps := []string{}
		switch ig.GetSubKind() {
		case types.IntegrationSubKindAWSOIDC:
			specProps = append(specProps, fmt.Sprintf("RoleARN=%s", ig.GetAWSOIDCIntegrationSpec().RoleARN))
		}

		rows = append(rows, []string{
			ig.GetName(), ig.GetSubKind(), strings.Join(specProps, ","),
		})
	}
	headers := []string{"Name", "Type", "Spec"}
	t := asciitable.MakeTable(headers, rows...)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
