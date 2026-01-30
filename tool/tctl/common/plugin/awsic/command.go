package awsic

import (
	"context"

	"github.com/alecthomas/kingpin/v2"
	apicommon "github.com/gravitational/teleport/api/types/common"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	"github.com/gravitational/trace"
)

type dependencies struct {
	parent         *Command
	clientProvider commonclient.InitFunc
}

type tryRunner interface {
	TryRun(context.Context, string, *dependencies) (bool, error)
}

type Command struct {
	cmd       *kingpin.CmdClause
	Name      string
	Resources ResourcesCmd
}

func (c *Command) Init(parent *kingpin.CmdClause) {
	cmd := parent.Command("awsic", "Manage the AWS Identity Center plugin")

	c.cmd = cmd
	cmd.Flag("plugin-name", "Name of the AWSIC plugin instance to manipulate").
		Default(apicommon.OriginAWSIdentityCenter).
		StringVar(&c.Name)

	c.Resources.init(cmd)
}

func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	handlers := []tryRunner{
		&c.Resources,
	}

	deps := dependencies{
		parent:         c,
		clientProvider: clientFunc,
	}

	for _, handler := range handlers {
		if matched, err := handler.TryRun(ctx, cmd, &deps); matched {
			return matched, trace.Wrap(err)
		}
	}

	return false, nil
}
