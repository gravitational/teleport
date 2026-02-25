package common

import (
	"context"
	"os"
	"os/exec"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

func UpdateHostCompletions(ctx context.Context, tc *client.TeleportClient) error {
	allResources := make(map[string][]string)
	const hostKey = "nodes_by_hostname"
	nodes, err := tc.ListNodesWithFilters(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	nodeHosts := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeHosts = append(nodeHosts, node.GetHostname())
	}
	allResources[hostKey] = nodeHosts
	return nil
}

func UpdateCompletionsInBackground() error {
	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command(executable, "update-completions")
	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(cmd.Process.Release())
}
