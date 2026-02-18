package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type beamsExposeCommand struct {
	*kingpin.CmdClause
	name        string
	mode        string
	port        int
	serviceName string
}

func newBeamsExposeCommand(parent *kingpin.CmdClause) *beamsExposeCommand {
	cmd := &beamsExposeCommand{
		CmdClause: parent.Command("expose", "Serve an HTTP or TCP service running in a beam environment."),
	}
	cmd.Arg("name", "Name of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Flag("mode", "Transport mode for the connection.").Default("http").StringVar(&cmd.mode)
	cmd.Flag("port", "Local beam environment port to expose.").Default("8080").IntVar(&cmd.port)
	cmd.Flag("name", "Name of the exposed application (optional).").StringVar(&cmd.serviceName)
	return cmd
}

func (c *beamsExposeCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.ExposeBeam(ctx, c.name, c.mode, c.port, c.serviceName)

	return trace.Wrap(err)
}
