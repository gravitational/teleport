package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsAddCommand struct {
	*kingpin.CmdClause
	name string
	wait bool
}

func newBeamsAddCommand(parent *kingpin.CmdClause) *beamsAddCommand {
	cmd := &beamsAddCommand{
		CmdClause: parent.Command("add", "Start a new beam environment."),
	}
	cmd.Arg("name", "Name for the new beam (optional).").StringVar(&cmd.name)
	cmd.Flag("wait", "Wait for the beam environment to become ready.").BoolVar(&cmd.wait)
	return cmd
}

func (c *beamsAddCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = tc.AddBeam(ctx, c.name, c.wait)

	return trace.Wrap(err)
}
