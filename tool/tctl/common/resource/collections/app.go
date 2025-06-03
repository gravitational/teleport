package collections

import (
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

func NewAppServerCollection(servers []types.AppServer) ResourceCollection {
	return &appServerCollection{
		servers: servers,
	}
}

type appServerCollection struct {
	servers []types.AppServer
}

func (a *appServerCollection) Resources() (r []types.Resource) {
	for _, resource := range a.servers {
		r = append(r, resource)
	}
	return r
}

func (a *appServerCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, server := range a.servers {
		app := server.GetApp()
		labels := common.FormatLabels(app.GetAllLabels(), verbose)
		rows = append(rows, []string{
			server.GetHostname(), app.GetName(), app.GetProtocol(), app.GetPublicAddr(), app.GetURI(), labels, server.GetTeleportVersion(),
		})
	}
	var t asciitable.Table
	headers := []string{"Host", "Name", "Type", "Public Address", "URI", "Labels", "Version"}
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (a *appServerCollection) writeJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, a.servers)
}

func (a *appServerCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, a.servers)
}

func NewAppCollection(apps []types.Application) ResourceCollection {
	return &appCollection{
		apps: apps,
	}
}

type appCollection struct {
	apps []types.Application
}

func (c *appCollection) Resources() (r []types.Resource) {
	for _, resource := range c.apps {
		r = append(r, resource)
	}
	return r
}

func (c *appCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, app := range c.apps {
		labels := common.FormatLabels(app.GetAllLabels(), verbose)
		rows = append(rows, []string{
			app.GetName(), app.GetDescription(), app.GetURI(), app.GetPublicAddr(), labels, app.GetVersion(),
		})
	}
	headers := []string{"Name", "Description", "URI", "Public Address", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
