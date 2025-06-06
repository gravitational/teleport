package collections

import (
	"io"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

type reverseTunnelCollection struct {
	tunnels []types.ReverseTunnel
}

func NewReverseTunnelCollection(tunnels []types.ReverseTunnel) ResourceCollection {
	return &reverseTunnelCollection{tunnels: tunnels}
}

func (r *reverseTunnelCollection) Resources() (res []types.Resource) {
	for _, resource := range r.tunnels {
		res = append(res, resource)
	}
	return res
}

func (r *reverseTunnelCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Cluster Name", "Dial Addresses"})
	for _, tunnel := range r.tunnels {
		t.AddRow([]string{
			tunnel.GetClusterName(), strings.Join(tunnel.GetDialAddrs(), ","),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
