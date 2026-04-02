package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsRemoveCommand struct {
	*kingpin.CmdClause
	name string
}

func newBeamsRemoveCommand(parent *kingpin.CmdClause) *beamsRemoveCommand {
	cmd := &beamsRemoveCommand{
		CmdClause: parent.Command("rm", "Delete a running beam environment."),
	}
	cmd.Alias("remove")
	cmd.Arg("name", "Name of the beam to delete.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsRemoveCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.RemoveBeam(ctx, c.name)

	return trace.Wrap(err)
}
