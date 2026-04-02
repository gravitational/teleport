package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsShellCommand struct {
	*kingpin.CmdClause
	name string
}

func newBeamsShellCommand(parent *kingpin.CmdClause) *beamsShellCommand {
	cmd := &beamsShellCommand{
		CmdClause: parent.Command("shell", "Start an interactive shell in a running beam environment."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsShellCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.ShellBeam(ctx, c.name)

	return trace.Wrap(err)
}
