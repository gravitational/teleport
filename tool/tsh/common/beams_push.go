package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsPushCommand struct {
	*kingpin.CmdClause
	name   string
	local  string
	remote string
}

func newBeamsPushCommand(parent *kingpin.CmdClause) *beamsPushCommand {
	cmd := &beamsPushCommand{
		CmdClause: parent.Command("push", "Copy a local file to a running beam environment."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Flag("local", "Local file to copy.").Required().StringVar(&cmd.local)
	cmd.Flag("remote", "Remote copy location.").Required().StringVar(&cmd.remote)
	return cmd
}

func (c *beamsPushCommand) run(cf *CLIConf) error {
	// TODO: Use `tsh scp`

	return trace.NotImplemented("`tsh beams push` is stubbed")
}
