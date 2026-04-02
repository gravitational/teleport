package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsListCommand struct {
	*kingpin.CmdClause
	format   string
	allUsers bool
}

func newBeamsListCommand(parent *kingpin.CmdClause) *beamsListCommand {
	cmd := &beamsListCommand{
		CmdClause: parent.Command("ls", "List running beam environments."),
	}
	cmd.Alias("list")
	cmd.Flag("format", "Format output (text, json, yaml).").
		Short('f').
		Default("text").
		EnumVar(&cmd.format, "text", "json", "yaml")
	cmd.Flag("all", "List beams for all users.").BoolVar(&cmd.allUsers)
	return cmd
}

func (c *beamsListCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = tc.ListBeams(ctx, c.allUsers)

	return trace.Wrap(err)
}
