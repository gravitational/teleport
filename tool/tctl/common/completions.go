package common

import (
	"context"
	"os"
	"os/exec"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
	"github.com/gravitational/trace"
)

const completionCommand = "update-completions"

type CompletionCommand struct {
	app    *kingpin.Application
	comCmd *kingpin.CmdClause
}

func (c *CompletionCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	c.app = app
	c.comCmd = app.Command(completionCommand, "Update local completions cache").Hidden()
}

func (c *CompletionCommand) TryRun(ctx context.Context, cmd string, getClient commonclient.InitFunc) (match bool, err error) {
	switch cmd {
	case c.comCmd.FullCommand():
	default:
		return false, nil
	}
	clt, close, err := getClient(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer close(ctx)
	return true, trace.Wrap(UpdateResourceCompletions(ctx, clt))
}

func UpdateResourceCompletions(ctx context.Context, clt *authclient.Client) error {
	allResources := make(map[string][]string)
	for kind, handler := range resources.Handlers() {
		if handler.Get == nil {
			continue
		}
		resources, err := handler.Get(ctx, clt, services.Ref{
			Kind: kind,
		}, resources.GetOpts{})
		if err != nil {
			return trace.Wrap(err)
		}
		names := make([]string, 0, len(resources.Resources()))
		for _, resource := range resources.Resources() {
			names = append(names, resource.GetName())
		}
		allResources[kind] = names
	}

	// update file
	return nil
}

func UpdateCompletionsInBackground() error {
	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command(executable, completionCommand)
	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(cmd.Process.Release())
}
