package collections

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/trace"
	"io"
)

type serverCollection struct {
	servers []types.Server
}

func NewServerCollection(servers []types.Server) ResourceCollection {
	return &serverCollection{servers: servers}
}

func (s *serverCollection) Resources() (r []types.Resource) {
	for _, resource := range s.servers {
		r = append(r, resource)
	}
	return r
}

func (s *serverCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, se := range s.servers {
		labels := common.FormatLabels(se.GetAllLabels(), verbose)
		rows = append(rows, []string{
			se.GetHostname(), se.GetName(), se.GetAddr(), labels, se.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "UUID", "Public Address", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (s *serverCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, s.servers)
}

func (s *serverCollection) writeJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, s.servers)
}

type serverInfoCollection struct {
	serverInfos []types.ServerInfo
}

func NewServerInfoCollection(serverInfos []types.ServerInfo) ResourceCollection {
	return &serverInfoCollection{serverInfos: serverInfos}
}

func (c *serverInfoCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.serverInfos))
	for i, resource := range c.serverInfos {
		r[i] = resource
	}
	return r
}

func (c *serverInfoCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Labels"})
	for _, si := range c.serverInfos {
		t.AddRow([]string{si.GetName(), PrintMetadataLabels(si.GetNewLabels())})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
