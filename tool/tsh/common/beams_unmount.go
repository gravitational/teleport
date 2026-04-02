package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsUnmountCommand struct {
	*kingpin.CmdClause
	name string
	dest string
}

func newBeamsUnmountCommand(parent *kingpin.CmdClause) *beamsUnmountCommand {
	cmd := &beamsUnmountCommand{
		CmdClause: parent.Command("unmount", "Remove a mounted directory from a beam evironment."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Flag("dest", "Mount location in the beam's filesystem.").Required().StringVar(&cmd.dest)
	return cmd
}

func (c *beamsUnmountCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.UnmountBeam(ctx, c.name, c.dest)

	return trace.Wrap(err)
}
