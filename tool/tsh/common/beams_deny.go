package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsDenyCommand struct {
	*kingpin.CmdClause
	name   string
	domain string
}

func newBeamsDenyCommand(parent *kingpin.CmdClause) *beamsDenyCommand {
	cmd := &beamsDenyCommand{
		CmdClause: parent.Command("deny", "Remove access to an external domain."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Flag("domain", "Domain to deny access to.").Required().StringVar(&cmd.domain)
	return cmd
}

func (c *beamsDenyCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.DenyBeamDomain(ctx, c.name, c.domain)

	return trace.Wrap(err)
}
