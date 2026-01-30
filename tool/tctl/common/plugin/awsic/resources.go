package awsic

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
)

type ResourcesCmd struct {
	cmd *kingpin.CmdClause

	ls listResourcesCmd
}

func (cmd *ResourcesCmd) init(parent *kingpin.CmdClause) {
	root := parent.Command("resources", "Manipulate AWS Identity Center managed resources")

	cmd.cmd = root
	cmd.ls.init(root)
}

func (c *ResourcesCmd) TryRun(ctx context.Context, cmd string, deps *dependencies) (bool, error) {
	var handler func(context.Context, *dependencies) error

	switch cmd {
	case c.ls.cmd.FullCommand():
		handler = c.ls.Run

	default:
		return false, nil
	}

	return true, trace.Wrap(handler(ctx, deps))
}

type listResourcesCmd struct {
	cmd *kingpin.CmdClause
}

func (ls *listResourcesCmd) init(parent *kingpin.CmdClause) {
	cmd := parent.Command("ls", "List managed AWS resources")
	ls.cmd = cmd
}

func (ls *listResourcesCmd) Run(ctx context.Context, deps *dependencies) error {
	clt, closeClient, err := deps.clientProvider(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeClient(ctx)

	ers, err := client.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
		Kinds: []string{types.KindIdentityCenterResource},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var resources []*proto.IdentityCenterResource

	for _, er := range ers {
		switch r := er.ResourceWithLabels.(type) {
		case *proto.IdentityCenterResource:
			resources = append(resources, r)

		default:
			fmt.Printf("Unexpected resource type %T\n", er.ResourceWithLabels)
			continue
		}
	}

	renderResourceList(resources)

	return nil
}

func renderResourceList(resources []*proto.IdentityCenterResource) error {
	table := asciitable.MakeTable([]string{"Kind", "Name", "AWS Account", "Display"})
	for _, r := range resources {
		table.AddRow([]string{
			r.GetSubKind(),
			r.GetName(),
			r.GetAWSAccount(),
			r.GetDisplayName(),
		})
	}
	return trace.Wrap(table.WriteTo(os.Stdout))
}
