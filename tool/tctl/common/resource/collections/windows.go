package collections

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/trace"
	"io"
)

type windowsDesktopServiceCollection struct {
	services []types.WindowsDesktopService
}

func NewWindowsDesktopServiceCollection(services []types.WindowsDesktopService) ResourceCollection {
	return &windowsDesktopServiceCollection{services: services}
}

func (c *windowsDesktopServiceCollection) Resources() (r []types.Resource) {
	for _, resource := range c.services {
		r = append(r, resource)
	}
	return r
}

func (c *windowsDesktopServiceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Address", "Version"})
	for _, service := range c.services {
		addr := service.GetAddr()
		if addr == reversetunnelclient.LocalWindowsDesktop {
			addr = "<proxy tunnel>"
		}
		t.AddRow([]string{service.GetName(), addr, service.GetTeleportVersion()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type windowsDesktopCollection struct {
	desktops []types.WindowsDesktop
}

func NewWindowsDesktopCollection(desktops []types.WindowsDesktop) ResourceCollection {
	return &windowsDesktopCollection{desktops: desktops}
}

func (c *windowsDesktopCollection) Resources() (r []types.Resource) {
	for _, resource := range c.desktops {
		r = append(r, resource)
	}
	return r
}

func (c *windowsDesktopCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, d := range c.desktops {
		labels := common.FormatLabels(d.GetAllLabels(), verbose)
		rows = append(rows, []string{d.GetName(), d.GetAddr(), d.GetDomain(), labels})
	}
	headers := []string{"Name", "Address", "AD Domain", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *windowsDesktopCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.desktops)
}

func (c *windowsDesktopCollection) writeJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, c.desktops)
}

type dynamicWindowsDesktopCollection struct {
	desktops []types.DynamicWindowsDesktop
}

func NewDynamicWindowsDesktopCollection(desktops []types.DynamicWindowsDesktop) ResourceCollection {
	return &dynamicWindowsDesktopCollection{desktops: desktops}
}

func (c *dynamicWindowsDesktopCollection) Resources() (r []types.Resource) {
	r = make([]types.Resource, 0, len(c.desktops))
	for _, resource := range c.desktops {
		r = append(r, resource)
	}
	return r
}

func (c *dynamicWindowsDesktopCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, d := range c.desktops {
		labels := common.FormatLabels(d.GetAllLabels(), verbose)
		rows = append(rows, []string{d.GetName(), d.GetAddr(), d.GetDomain(), labels})
	}
	headers := []string{"Name", "Address", "AD Domain", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
