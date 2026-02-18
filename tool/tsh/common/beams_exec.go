package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsExecCommand struct {
	*kingpin.CmdClause
	name string
	cmd  string
	dir  string
}

func newBeamsExecCommand(parent *kingpin.CmdClause) *beamsExecCommand {
	cmd := &beamsExecCommand{
		CmdClause: parent.Command("exec", "Run a command in a running beam environment."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Flag("cmd", "Command to execute in the environment.").Required().StringVar(&cmd.cmd)
	cmd.Flag("dir", "Directory to run the command in (optional).").StringVar(&cmd.dir)
	return cmd
}

func (c *beamsExecCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = tc.ExecBeam(ctx, c.name, c.cmd, c.dir)

	return trace.Wrap(err)
}
