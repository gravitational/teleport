package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsMountCommand struct {
	*kingpin.CmdClause
	name   string
	source string
	dest   string
}

func newBeamsMountCommand(parent *kingpin.CmdClause) *beamsMountCommand {
	cmd := &beamsMountCommand{
		CmdClause: parent.Command("mount", "Mount a local directory in a beam environment."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Flag("source", "Path to a local directory.").Required().StringVar(&cmd.source)
	cmd.Flag("dest", "Mount location in the beam's filesystem.").Required().StringVar(&cmd.dest)
	return cmd
}

func (c *beamsMountCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.MountBeam(ctx, c.name, c.source, c.dest)

	return trace.Wrap(err)
}
