package collections

import (
	"fmt"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/trace"
	"io"
	"strconv"
)

type staticHostUserCollection struct {
	items []*userprovisioningpb.StaticHostUser
}

func NewStaticHostUserCollection(items []*userprovisioningpb.StaticHostUser) ResourceCollection {
	return &staticHostUserCollection{items: items}
}

func (c *staticHostUserCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

func (c *staticHostUserCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {

		for _, matcher := range item.Spec.Matchers {
			labelMap := label.ToMap(matcher.NodeLabels)
			labelStringMap := make(map[string]string, len(labelMap))
			for k, vals := range labelMap {
				labelStringMap[k] = fmt.Sprintf("[%s]", printSortedStringSlice(vals))
			}
			var uid string
			if matcher.Uid != 0 {
				uid = strconv.Itoa(int(matcher.Uid))
			}
			var gid string
			if matcher.Gid != 0 {
				gid = strconv.Itoa(int(matcher.Gid))
			}
			rows = append(rows, []string{
				item.GetMetadata().Name,
				common.FormatLabels(labelStringMap, verbose),
				matcher.NodeLabelsExpression,
				printSortedStringSlice(matcher.Groups),
				uid,
				gid,
			})
		}
	}
	headers := []string{"Login", "Node Labels", "Node Expression", "Groups", "Uid", "Gid"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Node Expression")
	}
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
