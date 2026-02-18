package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsRunCommand struct {
	*kingpin.CmdClause
	command string
	dir     string
}

func newBeamsRunCommand(parent *kingpin.CmdClause) *beamsRunCommand {
	cmd := &beamsRunCommand{
		CmdClause: parent.Command("run", "Run a command in an ephemaral beam environment."),
	}
	cmd.Arg("command", "Command to execute in the environment.").Required().StringVar(&cmd.command)
	cmd.Flag("dir", "Directory to run the command in (optional).").StringVar(&cmd.dir)
	return cmd
}

func (c *beamsRunCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	beam, err := tc.AddBeam(ctx, "", true)
	if err != nil {
		return trace.Wrap(err, "adding beam")
	}

	_, err = tc.ExecBeam(ctx, beam.Name, c.command, c.dir)
	if err != nil {
		return trace.Wrap(err, "executing beam command")
	}

	// TODO print command output

	err = tc.RemoveBeam(ctx, beam.Name)
	if err != nil {
		return trace.Wrap(err, "removing beam")
	}

	return trace.Wrap(err)
}
