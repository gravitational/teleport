package awsic

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	cmd                   *kingpin.CmdClause
	includeAccessProfiles bool
	includeRequestable    bool
}

func (ls *listResourcesCmd) init(parent *kingpin.CmdClause) {
	cmd := parent.Command("ls", "List managed AWS resources")
	ls.cmd = cmd

	cmd.Flag("include-access-profiles", "include the applicable access profiles for each resource").
		Short('l').
		BoolVar(&ls.includeAccessProfiles)

	cmd.Flag("requestable", "include requestable resources").
		Short('r').
		BoolVar(&ls.includeRequestable)
}

func (ls *listResourcesCmd) Run(ctx context.Context, deps *dependencies) error {
	clt, closeClient, err := deps.clientProvider(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeClient(ctx)

	ers, err := client.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
		Kinds:              []string{types.KindIdentityCenterResource},
		IncludeLogins:      ls.includeAccessProfiles,
		IncludeRequestable: ls.includeRequestable,
		UseSearchAsRoles:   ls.includeRequestable,
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

	ls.renderResourceList(resources)

	return nil
}

func (ls *listResourcesCmd) renderResourceList(resources []*proto.IdentityCenterResource) error {
	columns := []string{"Kind", "Name", "AWS Account", "Display"}
	if ls.includeAccessProfiles {
		columns = append(columns, "Access Profiles")
	}

	table := asciitable.MakeTable(columns)
	for _, r := range resources {
		row := []string{
			r.GetSubKind(),
			r.GetName(),
			r.GetAWSAccount(),
			r.GetDisplayName(),
		}

		if ls.includeAccessProfiles {
			var apNames []string
			for _, ap := range r.AccessProfiles {
				apNames = append(apNames, ap.Name)
			}

			row = append(row, strings.Join(apNames, ", "))
		}

		table.AddRow(row)
	}
	return trace.Wrap(table.WriteTo(os.Stdout))
}
