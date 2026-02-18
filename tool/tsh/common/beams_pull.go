package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsPullCommand struct {
	*kingpin.CmdClause
	name   string
	remote string
	local  string
}

func newBeamsPullCommand(parent *kingpin.CmdClause) *beamsPullCommand {
	cmd := &beamsPullCommand{
		CmdClause: parent.Command("pull", "Copy a file from a running beam environment to the local filesystem."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Flag("remote", "Remote file to copy.").Required().StringVar(&cmd.remote)
	cmd.Flag("local", "Local copy location.").Required().StringVar(&cmd.local)
	return cmd
}

func (c *beamsPullCommand) run(cf *CLIConf) error {
	// TODO: Use `tsh scp`

	return trace.NotImplemented("`tsh beams pull` is stubbed")
}
