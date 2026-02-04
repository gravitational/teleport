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

type unpackedResource[T any] struct {
	src      *types.EnrichedResource
	id       types.ResourceID
	resource T
}

func (ls *listResourcesCmd) Run(ctx context.Context, deps *dependencies) error {
	clt, closeClient, err := deps.clientProvider(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeClient(ctx)

	clusterInfo, err := clt.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	cname := clusterInfo.GetClusterName()

	ers, err := client.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
		Kinds:              []string{types.KindIdentityCenterResource},
		IncludeLogins:      ls.includeAccessProfiles,
		IncludeRequestable: ls.includeRequestable,
		UseSearchAsRoles:   ls.includeRequestable,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var resources []unpackedResource[*proto.IdentityCenterResource]

	for _, er := range ers {
		switch r := er.ResourceWithLabels.(type) {
		case *proto.IdentityCenterResource:
			resources = append(resources, unpackedResource[*proto.IdentityCenterResource]{
				src: er,
				id: types.ResourceID{
					ClusterName: cname,
					Kind:        r.GetKind(),
					Name:        r.GetName(),
				},
				resource: r,
			})

		default:
			fmt.Printf("Unexpected resource type %T\n", er.ResourceWithLabels)
			continue
		}
	}

	ls.renderResourceList(resources)

	return nil
}

func (ls *listResourcesCmd) renderResourceList(resources []unpackedResource[*proto.IdentityCenterResource]) error {
	var columns []string

	if ls.includeRequestable {
		columns = append(columns, "R")
	}

	columns = append(columns, "ID", "Kind", "AWS Account", "Display")

	if ls.includeAccessProfiles {
		columns = append(columns, "Access Profiles")
	}

	table := asciitable.MakeTable(columns)
	for _, res := range resources {
		r := res.resource

		var row []string
		if ls.includeRequestable {
			value := ""
			if res.src.RequiresRequest {
				value = "*"
			}
			row = append(row, value)
		}

		row = append(row,
			types.ResourceIDToString(res.id),
			r.GetSubKind(),
			r.GetAWSAccount(),
			r.GetDisplayName(),
		)

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
