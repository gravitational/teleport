package common

import (
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/trace"
)

type vnetCLICommand interface {
	tryRun(cf *CLIConf, command string) (bool, error)
}

type vnetCommands struct {
	subcommands []vnetCLICommand
}

func (c *vnetCommands) tryRun(cf *CLIConf, command string) (bool, error) {
	for _, subcommand := range c.subcommands {
		if ok, err := subcommand.tryRun(cf, command); ok || err != nil {
			return ok, trace.Wrap(err)
		}
	}
	return false, nil
}

type vnetCommand struct {
	*kingpin.CmdClause
}

func newVnetCommand(app *kingpin.Application) *vnetCommand {
	cmd := &vnetCommand{
		CmdClause: app.Command("vnet", "Start Teleport VNet, a virtual network for TCP application access."),
	}
	return cmd
}

func (c *vnetCommand) tryRun(cf *CLIConf, command string) (bool, error) {
	if c.FullCommand() != command {
		return false, nil
	}
	return true, trace.Wrap(c.run(cf))
}

func (c *vnetCommand) run(cf *CLIConf) error {
	appProvider, err := newVnetAppProvider(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	processManager, err := vnet.Run(cf.Context, &vnet.RunConfig{AppProvider: appProvider})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		<-cf.Context.Done()
		processManager.Close()
	}()
	fmt.Println("VNet is ready.")
	return trace.Wrap(processManager.Wait())
}
