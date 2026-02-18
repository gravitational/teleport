package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsAllowCommand struct {
	*kingpin.CmdClause
	name   string
	domain string
}

func newBeamsAllowCommand(parent *kingpin.CmdClause) *beamsAllowCommand {
	cmd := &beamsAllowCommand{
		CmdClause: parent.Command("allow", "Grant access to an external domain."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Flag("domain", "Domain to allow access to.").Required().StringVar(&cmd.domain)
	return cmd
}

func (c *beamsAllowCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.AllowBeamDomain(ctx, c.name, c.domain)

	return trace.Wrap(err)
}
