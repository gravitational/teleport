package collections

import (
	"fmt"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"io"
)

type installerCollection struct {
	installers []types.Installer
}

func NewInstallerCollection(installers []types.Installer) ResourceCollection {
	return &installerCollection{installers: installers}
}

func (c *installerCollection) Resources() []types.Resource {
	var r []types.Resource
	for _, inst := range c.installers {
		r = append(r, inst)
	}
	return r
}

func (c *installerCollection) WriteText(w io.Writer, verbose bool) error {
	for _, inst := range c.installers {
		if _, err := fmt.Fprintf(w, "Script: %s\n----------\n", inst.GetName()); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(w, inst.GetScript()); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(w, "----------"); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
