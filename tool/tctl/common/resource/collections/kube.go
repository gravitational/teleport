package collections

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/trace"
	"io"
)

type kubeServerCollection struct {
	servers []types.KubeServer
}

func NewKubeServerCollection(servers []types.KubeServer) ResourceCollection {
	return &kubeServerCollection{servers: servers}
}

func (c *kubeServerCollection) Resources() (r []types.Resource) {
	for _, resource := range c.servers {
		r = append(r, resource)
	}
	return r
}

func (c *kubeServerCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, server := range c.servers {
		kube := server.GetCluster()
		if kube == nil {
			continue
		}
		labels := common.FormatLabels(kube.GetAllLabels(), verbose)
		rows = append(rows, []string{
			common.FormatResourceName(kube, verbose),
			labels,
			server.GetTeleportVersion(),
		})

	}
	headers := []string{"Cluster", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by cluster name.
	t.SortRowsBy([]int{0}, true)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *kubeServerCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.servers)
}

func (c *kubeServerCollection) writeJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, c.servers)
}

type kubeClusterCollection struct {
	clusters []types.KubeCluster
}

func NewKubeClusterCollection(clusters []types.KubeCluster) ResourceCollection {
	return &kubeClusterCollection{clusters: clusters}
}

func (c *kubeClusterCollection) Resources() (r []types.Resource) {
	for _, resource := range c.clusters {
		r = append(r, resource)
	}
	return r
}

// writeText formats the dynamic kube clusters into a table and writes them into w.
// Name          Labels
// ------------- ----------------------------------------------------------------------------------------------------------
// cluster1      region=eastus,resource-group=cluster1,subscription-id=subID
// cluster2      region=westeurope,resource-group=cluster2,subscription-id=subID
// cluster3      region=northcentralus,resource-group=cluster3,subscription-id=subID
// cluster4      owner=cluster4,region=southcentralus,resource-group=cluster4,subscription-id=subID
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *kubeClusterCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, cluster := range c.clusters {
		labels := common.FormatLabels(cluster.GetAllLabels(), verbose)
		rows = append(rows, []string{
			common.FormatResourceName(cluster, verbose),
			labels,
		})
	}
	headers := []string{"Name", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
