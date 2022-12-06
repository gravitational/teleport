package loginrule

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/e/lib/loginrule"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type subcommand interface {
	initialize(parent *kingpin.CmdClause)
	tryRun(ctx context.Context, selectedCommand string, c auth.ClientI) (match bool, err error)
}

// Command implements all commands under "tctl login_rule".
type Command struct {
	subcommands []subcommand
}

// Initialize installs the base "login_rule" command and all subcommands.
func (t *Command) Initialize(app *kingpin.Application, cfg *service.Config) {
	loginRuleCommand := app.Command("login_rule", "Manage cluster login rules").Hidden()

	t.subcommands = []subcommand{
		&testCommand{},
	}

	for _, subcommand := range t.subcommands {
		subcommand.initialize(loginRuleCommand)
	}
}

// TryRun calls tryRun for each subcommand, and if none of them match returns
// (false, nil)
func (t *Command) TryRun(ctx context.Context, selectedCommand string, c auth.ClientI) (match bool, err error) {
	for _, subcommand := range t.subcommands {
		match, err = subcommand.tryRun(ctx, selectedCommand, c)
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
	cmd                *kingpin.CmdClause
	inputResourceFiles []string
	inputTraitsFile    string
	outputFormat       string
}

func (t *testCommand) initialize(parent *kingpin.CmdClause) {
	t.cmd = parent.Command("test", "Test the parsing and evaluation of login rules before loading them into your cluster")
	t.cmd.Flag("resource-file", "login rule resource file name (YAML or JSON)").Required().StringsVar(&t.inputResourceFiles)
	t.cmd.Flag("format", "Output format: 'yaml' or 'json'").Default(teleport.YAML).StringVar(&t.outputFormat)
	t.cmd.Arg("traits-file", "input user traits file name (YAML or JSON), empty for stdin").StringVar(&t.inputTraitsFile)

	// Hack: use Alias to include some examples in the help output. This is also
	// done elsewhere in the codebase.
	t.cmd.Alias(`
Examples:

  Test evaluation of the login rules from rule1.yaml and rule2.yaml with input traits from traits.json

  > tctl login_rule test --resource-file rule1.yaml --resource-file rule2.yaml traits.json

  Read the input traits from stdin

  > echo '{"groups": ["example"]}' | tctl login_rule test --resource-file login_rule.yaml`)
}

func (t *testCommand) tryRun(ctx context.Context, selectedCommand string, c auth.ClientI) (match bool, err error) {
	if selectedCommand != t.cmd.FullCommand() {
		return false, nil
	}

	return true, trace.Wrap(t.run(ctx, c))
}

func (t *testCommand) run(ctx context.Context, c auth.ClientI) error {
	loginRules, err := parseLoginRuleFiles(t.inputResourceFiles)
	if err != nil {
		return trace.Wrap(err)
	}

	traits, err := parseTraitsFile(t.inputTraitsFile)
	if err != nil {
		return trace.Wrap(err)
	}

	result, err := loginrule.Evaluate(loginRules, &loginrule.EvaluationInput{Traits: traits})
	if err != nil {
		return trace.Wrap(err)
	}

	switch t.outputFormat {
	case teleport.YAML:
		utils.WriteYAML(os.Stdout, result.Traits)
	case teleport.JSON:
		utils.WriteJSON(os.Stdout, result.Traits)
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

		if raw.Kind != ResourceKind {
			return nil, trace.BadParameter("found resource kind %q, expected %s", raw.Kind, ResourceKind)
		}
		rule, err := unmarshalLoginRule(raw.Raw)
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
