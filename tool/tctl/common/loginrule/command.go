/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package loginrule

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type subcommand interface {
	initialize(parent *kingpin.CmdClause, cfg *servicecfg.Config)
	tryRun(ctx context.Context, selectedCommand string, clientFunc commonclient.InitFunc) (match bool, err error)
}

// Command implements all commands under "tctl login_rule".
type Command struct {
	subcommands []subcommand
}

// Initialize installs the base "login_rule" command and all subcommands.
func (t *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) {
	loginRuleCommand := app.Command("login_rule", "Test login rules")

	t.subcommands = []subcommand{
		&testCommand{},
	}

	for _, subcommand := range t.subcommands {
		subcommand.initialize(loginRuleCommand, cfg)
	}
}

// TryRun calls tryRun for each subcommand, and if none of them match returns
// (false, nil)
func (t *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	for _, subcommand := range t.subcommands {
		match, err = subcommand.tryRun(ctx, cmd, clientFunc)
		if err != nil {
			return match, trace.Wrap(err)
		}
		if match {
			return match, nil
		}
	}
	return false, nil
}

// testCommand implements the "tctl login_rule test" command.
type testCommand struct {
	cmd *kingpin.CmdClause

	inputResourceFiles []string
	loadFromCluster    bool
	inputTraitsFile    string
	outputFormat       string
}

func (t *testCommand) initialize(parent *kingpin.CmdClause, cfg *servicecfg.Config) {
	t.cmd = parent.Command("test", "Test the parsing and evaluation of login rules.")
	t.cmd.Flag("resource-file", "login rule resource file name (YAML or JSON)").StringsVar(&t.inputResourceFiles)
	t.cmd.Flag("load-from-cluster", "load existing login rules from the connected Teleport cluster").BoolVar(&t.loadFromCluster)
	t.cmd.Flag("format", "Output format: 'yaml' or 'json'").Default(teleport.YAML).StringVar(&t.outputFormat)
	t.cmd.Arg("traits-file", "input user traits file name (YAML or JSON), empty for stdin").StringVar(&t.inputTraitsFile)

	// Hack: use Alias to include some examples in the help output. This is also
	// done elsewhere in the codebase.
	t.cmd.Alias(`
Examples:

  Test evaluation of the login rules from rule1.yaml and rule2.yaml with input traits from traits.json

  > tctl login_rule test --resource-file rule1.yaml --resource-file rule2.yaml traits.json

  Test the login rule in rule.yaml along with all login rules already present in the cluster

  > tctl login_rule test --resource-file rule.yaml --load-from-cluster traits.json

  Read the input traits from stdin

  > echo '{"groups": ["example"]}' | tctl login_rule test --resource-file rule.yaml`)
}

func (t *testCommand) tryRun(ctx context.Context, selectedCommand string, clientFunc commonclient.InitFunc) (match bool, err error) {
	if selectedCommand != t.cmd.FullCommand() {
		return false, nil
	}

	if len(t.inputResourceFiles) == 0 && !t.loadFromCluster {
		return true, trace.BadParameter("no login rules to test, --resource-file or --load-from-cluster must be set")
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(t.run(ctx, client))
}

func (t *testCommand) run(ctx context.Context, c *authclient.Client) error {
	loginRules, err := parseLoginRuleFiles(t.inputResourceFiles)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(loginRules) == 0 && !t.loadFromCluster {
		return trace.BadParameter("no login rules to test")
	}

	if len(t.inputResourceFiles) > 0 {
		slog.DebugContext(ctx, "Loaded login rule(s) from input resource files", "login_rule_count", len(loginRules))
	}

	traits, err := parseTraitsFile(t.inputTraitsFile)
	if err != nil {
		return trace.Wrap(err)
	}

	result, err := c.LoginRuleClient().TestLoginRule(ctx, &loginrulepb.TestLoginRuleRequest{
		LoginRules:      loginRules,
		Traits:          traitsMapResourceToProto(traits),
		LoadFromCluster: t.loadFromCluster,
	})
	if err != nil {
		if trace.IsNotImplemented(err) {
			return trace.NotImplemented("the server does not support testing login rules - try downgrading your client to match the server version")
		}

		return trace.Wrap(err)
	}

	switch t.outputFormat {
	case teleport.YAML:
		utils.WriteYAML(os.Stdout, traitsMapProtoToResource(result.Traits))
	case teleport.JSON:
		utils.WriteJSONObject(os.Stdout, traitsMapProtoToResource(result.Traits))
	default:
		return trace.BadParameter("unsupported output format %q, supported values are %s and %s", t.outputFormat, teleport.YAML, teleport.JSON)
	}
	return nil
}

// parseLoginRuleFiles parses login rules from YAML or JSON files. Supports
// multiple rules per YAML file separated into YAML documents with "---".
func parseLoginRuleFiles(fileNames []string) ([]*loginrulepb.LoginRule, error) {
	var rules []*loginrulepb.LoginRule
	for _, fileName := range fileNames {
		fileRules, err := parseLoginRuleFile(fileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		rules = append(rules, fileRules...)
	}
	return rules, nil
}

func parseLoginRuleFile(fileName string) ([]*loginrulepb.LoginRule, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer f.Close()

	rules, err := parseLoginRules(f)
	return rules, trace.Wrap(err)
}

func parseLoginRules(r io.Reader) ([]*loginrulepb.LoginRule, error) {
	var rules []*loginrulepb.LoginRule
	decoder := kyaml.NewYAMLOrJSONDecoder(r, defaults.LookaheadBufSize)
	for {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return rules, nil
			}
			return nil, trace.Wrap(err)
		}

		if raw.Kind != types.KindLoginRule {
			return nil, trace.BadParameter("found resource kind %q, expected %s", raw.Kind, types.KindLoginRule)
		}
		rule, err := UnmarshalLoginRule(raw.Raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		rules = append(rules, rule)
	}
}

func parseTraitsFile(fileName string) (map[string][]string, error) {
	var r io.Reader = os.Stdin
	if fileName != "" {
		f, err := os.Open(fileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer f.Close()
		r = f
	}

	decoder := kyaml.NewYAMLOrJSONDecoder(r, defaults.LookaheadBufSize)
	var traits map[string][]string
	err := decoder.Decode(&traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return traits, nil
}
