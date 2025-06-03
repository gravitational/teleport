package collections

import (
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/trace"
	"io"
)

type userTaskCollection struct {
	items []*usertasksv1.UserTask
}

func NewUserTaskCollection(tasks []*usertasksv1.UserTask) ResourceCollection {
	return &userTaskCollection{items: tasks}
}

func (c *userTaskCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

// writeText formats the user tasks into a table and writes them into w.
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *userTaskCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {
		labels := common.FormatLabels(item.GetMetadata().GetLabels(), verbose)
		rows = append(rows, []string{item.Metadata.GetName(), labels, item.Spec.TaskType, item.Spec.IssueType, item.Spec.GetIntegration()})
	}
	headers := []string{"Name", "Labels", "TaskType", "IssueType", "Integration"}
	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
