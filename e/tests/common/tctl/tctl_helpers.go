package tctl

// Package tctl allows integration test suites to easily invoke tctl as part of
// their tests. Using `tctl` requires calling [IsReExec] in the package-under-test's
// TestMain() in order to work; for example:
//
// func TestMain(m *testing.M) {
//   if runTctl, isTctl := tctl.IsReExec(); isTctl {
//     runTctl()
//     return
//   }
//
//   os.Exit(m.Run())
// }

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	tctlcommon "github.com/gravitational/teleport/tool/tctl/common"
)

const (
	// testBinTCTLTestEnv is the environment variable that will be set to
	// re-execute test as tctl binary.
	testBinTCTLTestEnv = "EXEC_TEST_BIN_TCTL_TEST"
)

const configTemplate = `
version: v3
teleport:
  data_dir: %s
auth_service:
  enabled: "yes"
  listen_addr: %s
`

// New creates and initializes a new instance of the `tctl` CLI runner. Instances
// must be cleaned up with [CLI.Cleanup()]. One CLI instance can be used for
// multiple `tctl` runs.
func New(dataDir, listenAddr string) (result *CLI, err error) {
	tempDir, err := os.MkdirTemp("", "tctl-test-*")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	configFileName := filepath.Join(tempDir, "teleport.yaml")
	result = &CLI{
		configDir:  tempDir,
		configFile: configFileName,
	}
	// If the constructor fails at any point after this, we want to ensure that
	// we delete our temp dir and release any system resources.
	defer func() {
		if err != nil {
			result.Cleanup()
		}
	}()

	yamlConfig := fmt.Sprintf(configTemplate, dataDir, listenAddr)
	if err := os.WriteFile(configFileName, []byte(yamlConfig), 0600); err != nil {
		return nil, trace.Wrap(err)
	}

	return result, nil
}

// CLI holds all of the common data required to run `tctl` from tests.
type CLI struct {
	configDir  string
	configFile string
}

// Run runs the `tctl` CLI in a child process with no special options
func (cli *CLI) Run(ctx context.Context, args ...string) error {
	return trace.Wrap(cli.Command().Run(ctx, args...))
}

// Cleanup  ensures that any system resources (e.g. temp files) are released
// and/or deleted.
func (cli *CLI) Cleanup(args ...string) error {
	return trace.Wrap(os.RemoveAll(cli.configDir))
}

// CommandOption describes a function that can be used to provide custom options
// for a specific `tctl` execution.
type CommandOption func(*Command)

// WithStdout is a [CommandOption] that redirects the `tctl` process' stdout
// stream to a custom [io.Writer].
func WithStdout(w io.Writer) CommandOption {
	return func(cmd *Command) {
		cmd.stdout = w
	}
}

// Command offers a point of customization for individual `tctl` executions
func (cli *CLI) Command(options ...CommandOption) *Command {
	cmd := &Command{CLI: cli, stdout: os.Stdout}
	for _, applyOption := range options {
		applyOption(cmd)
	}
	return cmd
}

// Command holds configuration info for a single `tctl` invocation
type Command struct {
	*CLI
	stdout io.Writer
}

// Run executes `tctl` with the supplied command-line args
func (t *Command) Run(ctx context.Context, args ...string) error {
	thisExe, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}

	args = append(args,
		fmt.Sprintf(`--config=%s`, t.configFile),
	)

	child := exec.CommandContext(ctx, thisExe, args...)
	child.Stdout = t.stdout
	child.Stderr = os.Stderr
	child.Env = append(child.Env, fmt.Sprintf("%s=1", testBinTCTLTestEnv))

	return trace.Wrap(child.Run())
}

// IsReExec checks to see if the current process should be running as if it were
// `tctl`, and returns a function to execute containing the `tctl` implementation.
// Use [IsReExec] in the TestMain of your package to invoke `tctl` in tests.
func IsReExec() (func(), bool) {
	if os.Getenv(testBinTCTLTestEnv) == "" {
		return nil, false
	}

	runTctl := func() {
		if err := os.Setenv(types.HomeEnvVar, os.TempDir()); err != nil {
			utils.FatalError(err)
		}
		tctlcommon.Run(context.Background(), tctlcommon.Commands())
	}

	return runTctl, true
}
