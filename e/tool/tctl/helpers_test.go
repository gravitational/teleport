package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	tctl "github.com/gravitational/teleport/tool/tctl/common"
	aclcommand "github.com/gravitational/teleport/tool/tctl/common/accesslist"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/integrations"
)

func runCommand(t require.TestingT, client *authclient.Client, cmd tctl.CLICommand, args []string) error {
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	app := utils.InitCLIParser("tctl", tctl.GlobalHelpString)
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	selectedCmd, err := app.Parse(args)
	require.NoError(t, err)

	_, err = cmd.TryRun(context.Background(), selectedCmd, func(ctx context.Context) (*authclient.Client, func(context.Context), error) {
		return client, func(context.Context) {}, nil
	})
	return err
}

func runDevicesCommand(t *testing.T, client *authclient.Client, args []string) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &tctl.DevicesCommand{
		Stdout: &stdoutBuff,
	}

	args = append([]string{"devices"}, args...)
	return &stdoutBuff, runCommand(t, client, command, args)
}

func runACLCommand(t *testing.T, client *authclient.Client, args []string) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &aclcommand.Command{
		Stdout: &stdoutBuff,
	}

	args = append([]string{"acl"}, args...)
	return &stdoutBuff, runCommand(t, client, command, args)
}

func runResourceCommand(t *testing.T, client *authclient.Client, args []string) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &tctl.ResourceCommand{
		Stdout: &stdoutBuff,
	}

	return &stdoutBuff, runCommand(t, client, command, args)
}

func runAWSICCommand(t *testing.T, client *authclient.Client, args []string) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &integrations.Command{
		Stdout: &stdoutBuff,
	}

	args = append([]string{"integrations", "awsic"}, args...)
	return &stdoutBuff, runCommand(t, client, command, args)
}

func mustDecodeJSON[T any](t *testing.T, r io.Reader) T {
	var out T
	err := json.NewDecoder(r).Decode(&out)
	require.NoError(t, err)
	return out
}

func mustTranscodeYAMLToJSON(t *testing.T, r io.Reader) []byte {
	decoder := kyaml.NewYAMLToJSONDecoder(r)
	var resource services.UnknownResource
	require.NoError(t, decoder.Decode(&resource))
	return resource.Raw
}

// mustTranscodeYAMLDocsToJSON safely transcodes YAML docs for unknown resources.
func mustTranscodeYAMLDocsToJSON(t *testing.T, r io.Reader) []byte {
	decoder := kyaml.NewYAMLToJSONDecoder(r)
	var jsonRaw []json.RawMessage

	for {
		var resource services.UnknownResource
		if err := decoder.Decode(&resource); err != nil {
			// Break when there are no more documents to decode
			if !errors.Is(err, io.EOF) {
				require.FailNow(t, "error transcoding YAML docs to JSON: %v", err)
			}
			break
		}
		jsonRaw = append(jsonRaw, resource.Raw)
	}

	jsonDocs, err := json.Marshal(jsonRaw)
	require.NoError(t, err)

	return jsonDocs
}
