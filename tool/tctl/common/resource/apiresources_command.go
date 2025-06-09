package resource

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type APIResourcesCommand struct {
	app    *kingpin.Application
	cmd    *kingpin.CmdClause
	config *servicecfg.Config

	stdout io.Writer
}

func (c *APIResourcesCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.app = app
	c.config = config
	c.cmd = app.Command("api-resources", "Lists the tctl-supported resources")

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

func (c *APIResourcesCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	if cmd != c.cmd.FullCommand() {
		return false, nil
	}

	t := asciitable.MakeTable([]string{"Kind", "Supported Commands", "Singleton", "Description"})
	for kind, handler := range resourceHandlers {
		t.AddRow([]string{
			string(kind),
			strings.Join(handler.supportedCommands(), ","),
			strconv.FormatBool(handler.singleton),
			handler.description,
		})
	}

	t.SortRowsBy([]int{0}, true)

	return true, trace.Wrap(t.WriteTo(c.stdout))
}
