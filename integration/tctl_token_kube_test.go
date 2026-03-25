/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/buildkite/bintest/v3"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integrations/lib/embeddedtbot"
	"github.com/gravitational/teleport/lib/kube/token"
	"github.com/gravitational/teleport/lib/oidc/fakeissuer"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/tctl/common"
)

func setupCLusterForTCTLTokensKubeTests(t *testing.T, log *slog.Logger) (*helpers.TeleInstance, string, string) {
	// This test is not Parallel because of the metrics black hole.
	testDir := t.TempDir()
	prometheus.DefaultRegisterer = metricRegistryBlackHole{}

	// Test setup: creating a teleport instance running auth and proxy
	clusterName := "root.example.com"
	cfg := helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      log.With("test_component", "test-instance"),
	}
	cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = filepath.Join(testDir, "data")
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.SSH.Enabled = false
	rcConf.Proxy.DisableWebInterface = true
	rcConf.Version = "v3"
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)

	// Test setup: create a test user that can create tokens.
	testUsername := "test-user"
	role, err := types.NewRole("test-role", types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindToken, types.KindBot},
					Verbs:     []string{types.VerbRead, types.VerbCreate, types.VerbList, types.VerbUpdate},
				},
			},
		},
	})
	require.NoError(t, err)
	rc.AddUserWithRole(testUsername, role)

	// Test setup: starting the Teleport instance
	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, rc.StopAll())
	})

	return rc, testUsername, role.GetName()
}

// TestTCTLTokenConfigureKubeCommand_OIDC validates that the command `tctl tokens configure-kube --join-with oidc`
// can run against a fake Kubernetes cluster, a Teleport cluster service and
// generates a join token that allows workload from the kubernetes cluster to join.
func TestTCTLTokenConfigureKubeCommand_OIDC(t *testing.T) {
	const (
		testKubeContext = "test-context"
		testNamespace   = "test-namespace"
		testRelease     = "test-release"
		testBotName     = "test-bot"
	)

	log := logtest.NewLogger()

	rc, testUsername, testRoleName := setupCLusterForTCTLTokensKubeTests(t, log)

	// Test setup: obtaining and authclient connected via the proxy.
	clt := getAuthClientForProxy(t, rc, testUsername, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pong, err := clt.Ping(ctx)
	require.NoError(t, err)

	_, err = clt.BotServiceClient().CreateBot(ctx, &machineidv1.CreateBotRequest{
		Bot: &machineidv1.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: testBotName,
			},
			Spec: &machineidv1.BotSpec{
				Roles: []string{testRoleName},
			},
		},
	})
	require.NoError(t, err)

	addr, err := rc.Process.ProxyWebAddr()
	require.NoError(t, err)

	// Test setup: creating a fake Kubernetes OIDC provider.
	idp, err := fakeissuer.NewIDP(log.With("test_component", "fakeissuer"))
	require.NoError(t, err)
	kubectlOIDConfig := struct {
		Issuer                 string   `json:"issuer"`
		JWKSURI                string   `json:"jwks_uri"`
		ResponseTypesSupported []string `json:"response_types_supported"`
		SubjectTypesSupported  []string `json:"subject_types_supported"`
		IDTokenSigningAlgs     []string `json:"id_token_signing_algs_values_supported"`
	}{
		Issuer:                 idp.IssuerURL(),
		JWKSURI:                "not currently used",
		ResponseTypesSupported: []string{"id_token"},
		SubjectTypesSupported:  []string{"public"},
		IDTokenSigningAlgs:     []string{"RS256"},
	}
	kubectlOIDConfigResponse, err := json.Marshal(kubectlOIDConfig)
	require.NoError(t, err)

	// Test setup: creating a mock kubectl binary simulating our cluster.
	kubectlMock, err := bintest.NewMock("kubectl")
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, kubectlMock.Close())
	})

	kubectlMock.Expect("config", "current-context").AndWriteToStdout(testKubeContext)
	kubectlMock.Expect("--context", testKubeContext, "get", "--raw=/.well-known/openid-configuration").AndWriteToStdout(string(kubectlOIDConfigResponse))

	// Test execution: create the tctl command.
	stdout := &bytes.Buffer{}
	tctlCfg := &servicecfg.Config{}
	err = tctlCfg.SetAuthServerAddresses([]utils.NetAddr{*addr})
	require.NoError(t, err)
	tctlCommand := common.TokensCommand{
		Stdout:     stdout,
		BinKubectl: kubectlMock.Path,
	}

	tempDir := t.TempDir()
	app := kingpin.New("test", "test")
	tctlCommand.Initialize(app, nil, tctlCfg)
	_, err = app.Parse([]string{"tokens", "configure-kube", "--join-with", "oidc", "-s", testRelease, "--bot", testBotName, "--namespace", testNamespace, "--out", tempDir + "/values.yaml"})
	require.NoError(t, err)

	// Test execution: run the command and validate it doesn't error.
	err = tctlCommand.ConfigureKube(ctx, clt)
	require.NoError(t, err)

	// Test validation: obtain a kubernetes JWT from our fake IDP.
	kubeToken, err := idp.IssueKubeToken("random-pod-name", testNamespace, testRelease, pong.ClusterName)
	require.NoError(t, err)

	tokenPath := tempDir + "/token"
	t.Setenv(token.EnvVarCustomKubernetesTokenPath, tokenPath)
	require.NoError(t, os.WriteFile(tokenPath, []byte(kubeToken), 0600))

	// Test validation: configure a bot to join using our JWT and the token created by the tctl command.
	botConfig := &embeddedtbot.BotConfig{
		AuthServer: rc.Auth,
		Onboarding: onboarding.Config{
			TokenValue: testKubeContext + "-" + testBotName,
			JoinMethod: types.JoinMethodKubernetes,
		},
		CredentialLifetime: bot.CredentialLifetime{
			TTL:             time.Hour,
			RenewalInterval: time.Hour,
		},
		Insecure: true,
	}

	// Test validation: run the bot and make sure it can join the cluster.
	bot, err := embeddedtbot.New(botConfig, log.With("test_component", "tbot"))
	require.NoError(t, err)
	_, err = bot.Preflight(t.Context())
	require.NoError(t, err)
}

// TestTCTLTokenConfigureKubeCommand_JWKS validates that the command `tctl tokens configure-kube --join-with jwks`
// can run against a fake Kubernetes cluster, a Teleport cluster service and
// generates a join token that allows workload from the kubernetes cluster to join.
func TestTCTLTokenConfigureKubeCommand_JWKS(t *testing.T) {
	const (
		testKubeContext = "test-context"
		testNamespace   = "test-namespace"
		testRelease     = "test-release"
		testBotName     = "test-bot"
	)

	log := logtest.NewLogger()

	rc, testUsername, testRoleName := setupCLusterForTCTLTokensKubeTests(t, log)

	// Test setup: obtaining and authclient connected via the proxy.
	clt := getAuthClientForProxy(t, rc, testUsername, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pong, err := clt.Ping(ctx)
	require.NoError(t, err)

	_, err = clt.BotServiceClient().CreateBot(ctx, &machineidv1.CreateBotRequest{
		Bot: &machineidv1.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: testBotName,
			},
			Spec: &machineidv1.BotSpec{
				Roles: []string{testRoleName},
			},
		},
	})
	require.NoError(t, err)

	addr, err := rc.Process.ProxyWebAddr()
	require.NoError(t, err)

	// Test setup: creating a fake Kubernetes OIDC provider.
	signer, err := fakeissuer.NewKubernetesSigner(clockwork.NewRealClock())
	require.NoError(t, err)
	kubectlJWKSResponse, err := signer.GetMarshaledJWKS()
	require.NoError(t, err)

	// Test setup: creating a mock kubectl binary simulating our cluster.
	kubectlMock, err := bintest.NewMock("kubectl")
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, kubectlMock.Close())
	})

	kubectlMock.Expect("config", "current-context").AndWriteToStdout(testKubeContext)
	kubectlMock.Expect("--context", testKubeContext, "get", "--raw=/openid/v1/jwks").AndWriteToStdout(kubectlJWKSResponse)

	// Test execution: create the tctl command.
	stdout := &bytes.Buffer{}
	tctlCfg := &servicecfg.Config{}
	err = tctlCfg.SetAuthServerAddresses([]utils.NetAddr{*addr})
	require.NoError(t, err)
	tctlCommand := common.TokensCommand{
		Stdout:     stdout,
		BinKubectl: kubectlMock.Path,
	}

	tempDir := t.TempDir()
	app := kingpin.New("test", "test")
	tctlCommand.Initialize(app, nil, tctlCfg)
	_, err = app.Parse([]string{"tokens", "configure-kube", "--join-with", "jwks", "-s", testRelease, "--bot", testBotName, "--namespace", testNamespace, "--out", tempDir + "/values.yaml"})
	require.NoError(t, err)

	// Test execution: run the command and validate it doesn't error.
	err = tctlCommand.ConfigureKube(ctx, clt)
	require.NoError(t, err)

	// Test validation: obtain a kubernetes JWT from our fake IDP.
	kubeToken, err := signer.SignServiceAccountJWT("random-pod-name", testNamespace, testRelease, pong.ClusterName)
	require.NoError(t, err)

	tokenPath := tempDir + "/token"
	t.Setenv(token.EnvVarCustomKubernetesTokenPath, tokenPath)
	require.NoError(t, os.WriteFile(tokenPath, []byte(kubeToken), 0600))

	// Test validation: configure a bot to join using our JWT and the token created by the tctl command.
	botConfig := &embeddedtbot.BotConfig{
		AuthServer: rc.Auth,
		Onboarding: onboarding.Config{
			TokenValue: testKubeContext + "-" + testBotName,
			JoinMethod: types.JoinMethodKubernetes,
		},
		CredentialLifetime: bot.CredentialLifetime{
			TTL:             time.Hour,
			RenewalInterval: time.Hour,
		},
		Insecure: true,
	}

	// Test validation: run the bot and make sure it can join the cluster.
	bot, err := embeddedtbot.New(botConfig, log.With("test_component", "tbot"))
	require.NoError(t, err)
	_, err = bot.Preflight(t.Context())
	require.NoError(t, err)

}
