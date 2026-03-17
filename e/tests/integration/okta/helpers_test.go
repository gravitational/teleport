package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/e/tests/common/tctl"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

func sortByName[T interface{ GetName() string }](slice []T) {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].GetName() < slice[j].GetName()
	})
}

type resource interface {
	GetName() string
}

type resourceDesc interface {
	GetDescription() string
}

func assertResourcesByName[T resource](t assert.TestingT, want, got []T) {
	sortByName(want)
	sortByName(got)
	assert.Empty(t, cmp.Diff(want, got, cmp.Comparer(func(x, y resource) bool {
		return x.GetName() == y.GetName()
	}), cmpopts.EquateEmpty()))
}

func assertResourcesByDesc[T resourceDesc](t assert.TestingT, want, got []T) {
	assert.Empty(t, cmp.Diff(want, got, cmp.Comparer(func(x, y resourceDesc) bool {
		return x.GetDescription() == y.GetDescription()
	})))
}

func mustRunTCTLAndGetResultAs(t require.TestingT, tctlCLI *tctl.CLI, args []string, v any) {
	var output bytes.Buffer
	err := tctlCLI.
		Command(tctl.WithStdout(&output)).
		Run(context.Background(), args...)
	require.NoError(t, err)
	err = json.Unmarshal(output.Bytes(), v)
	require.NoError(t, err)
}

func mustRunTSHAndGetResultAs(t require.TestingT, tsh *tshCommand, args []string, v any) {
	var output bytes.Buffer
	err := tsh.run(t, args, withStdout(&output))
	require.NoError(t, err)
	err = json.Unmarshal(output.Bytes(), v)
	require.NoError(t, err)
}

func mustWaitForEvent(t *testing.T, sut *common.SUT, eventType string, opts ...waitOption) {
	options := &waitOptions{
		timeout:   1 * time.Minute,
		step:      100 * time.Millisecond,
		timePoint: time.Now(),
	}
	for _, o := range opts {
		o(options)
	}

	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		gotEvents, _, err := sut.Teleport.Process.GetAuthServer().SearchEvents(ctx, events.SearchEventsRequest{
			From: options.timePoint,
			To:   time.Now(),
			EventTypes: []string{
				eventType,
			},
		})
		require.NoError(t, err)
		ok := len(gotEvents) >= 1
		require.True(t, ok)
	}, options.timeout, options.step, "failed to wait for %s", eventType)
}

func userExistInTeleportAndIsNotLocked(t *testing.T, ctx context.Context, auth *auth.Server, oktaLogin string) {
	_, err := auth.GetUser(ctx, oktaLogin, false)
	require.NoError(t, err)
	locks, err := auth.GetLocks(ctx, false, types.LockTarget{User: oktaLogin})
	require.NoError(t, err)
	require.Empty(t, locks)
}

type cmdOptions struct {
	stdout io.Writer
}

type cmdOption func(*cmdOptions)

func withStdout(w io.Writer) cmdOption {
	return func(o *cmdOptions) {
		o.stdout = w
	}
}

type output struct {
	mtx sync.Mutex
	buf bytes.Buffer
}

func (o *output) Read(p []byte) (int, error) {
	o.mtx.Lock()
	defer o.mtx.Unlock()
	return o.buf.Read(p)
}

func (o *output) Write(p []byte) (int, error) {
	o.mtx.Lock()
	defer o.mtx.Unlock()
	return o.buf.Write(p)
}

func (o *output) String() string {
	o.mtx.Lock()
	defer o.mtx.Unlock()
	return o.buf.String()
}

type tshCommand struct {
	DataDir   string
	Listener  string
	out       output
	tshHome   string
	proxyAddr string
	clock     clockwork.Clock
}

func (c *tshCommand) mockIDPAuthFlow(t *testing.T, username string, groups []string) {
	go func() {
		var url string
		// Wait for tsh login --auth=okta login and read the URL from the output.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			re := regexp.MustCompile(`http://[^\s]+/[0-9a-fA-F-]{36}`)
			urls := re.FindAllString(c.out.String(), -1)
			require.Len(t, urls, 1)

			url = urls[0]
		}, time.Second*10, time.Millisecond*100)

		idpMock := idp.New(idp.Config{
			ProxyAddr: c.proxyAddr,
			Clock:     c.clock,
			URL:       url,
		})
		// Mock the IdP flow and return the username and group traits in SAML assertion.
		idpMock.Do(t, username, groups)
	}()
}

func (c *tshCommand) mustLogin(t *testing.T, username string, groups []string) {
	c.mockIDPAuthFlow(t, username, groups)
	err := c.run(t, []string{
		"login",
		fmt.Sprintf(`--proxy=%s`, c.proxyAddr),
		`--auth=okta-pre-created-test`,
		"--browser", "none",
		`--bind-addr=127.0.0.1:4444`,
		"--insecure",
		"-d",
	}, withStdout(&c.out))
	require.NoError(t, err)
}

func (c *tshCommand) run(t require.TestingT, args []string, opts ...cmdOption) error {
	options := &cmdOptions{
		stdout: os.Stdout,
	}
	for _, o := range opts {
		o(options)
	}
	selfExe, err := os.Executable()
	if err != nil {
		return err
	}
	execCmd := exec.Command(selfExe,
		args...,
	)
	execCmd.Stderr = options.stdout
	execCmd.Stdout = options.stdout
	execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=yes", testBinTSHTestEnv))
	execCmd.Env = append(execCmd.Env, "TELEPORT_HOME="+c.tshHome)
	err = execCmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func waitForOktaFirstOktaAssignment(t *testing.T, sut *common.SUT) {
	t.Helper()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assignments, _, err := sut.Teleport.Process.GetAuthServer().ListOktaAssignments(context.Background(), 0, "")
		assert.NoError(t, err)
		assert.NotEmpty(t, assignments)
	}, time.Second*10, time.Millisecond*100)
}

type waitOptions struct {
	timeout   time.Duration
	step      time.Duration
	timePoint time.Time
}

func withTimePoint(t time.Time) waitOption {
	return func(o *waitOptions) {
		o.timePoint = t
	}
}

type waitOption func(*waitOptions)

func withTimeout(t time.Duration) waitOption {
	return func(o *waitOptions) {
		o.timeout = t
	}
}

func withStep(t time.Duration) waitOption {
	return func(o *waitOptions) {
		o.step = t
	}
}

func waitForOktaSync(t *testing.T, sut *common.SUT, opts ...waitOption) {
	t.Helper()
	mustWaitForEvent(t, sut, events.OktaUserSyncEvent, opts...)
	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, opts...)
}

type integrationSettings struct {
	enableUserSync          bool
	enableAppGroupSync      bool
	enableAccessListSync    bool
	enableBidirectionalSync bool
}

type createIntegrationSettings struct {
	integrationSettings
	apiCredentials *oktav1.OktaAPICredentials
	reuseConnector string
}

func mustCreateIntegration(t *testing.T, oktaAuthClient oktav1.OktaServiceClient, settings createIntegrationSettings) {
	t.Helper()
	ctx := t.Context()

	_, err := oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		TimeBetweenImports:      durationpb.New(1 * time.Second),
		ApiCredentials:          settings.apiCredentials,
		ReuseConnector:          settings.reuseConnector,
		EnableUserSync:          settings.enableUserSync,
		EnableAppGroupSync:      settings.enableAppGroupSync,
		EnableAccessListSync:    settings.enableAccessListSync,
		EnableBidirectionalSync: settings.enableBidirectionalSync,
	})
	require.NoError(t, err)
}

func mustUpdateIntegration(t *testing.T, oktaAuthClient oktav1.OktaServiceClient, settings integrationSettings) {
	t.Helper()
	ctx := t.Context()

	mustUpdateOktaIntegration(ctx, t, oktaAuthClient, &oktav1.UpdateIntegrationRequest{
		TimeBetweenImports:      durationpb.New(1 * time.Second),
		EnableUserSync:          settings.enableUserSync,
		EnableAppGroupSync:      settings.enableAppGroupSync,
		EnableAccessListSync:    settings.enableAccessListSync,
		EnableBidirectionalSync: settings.enableBidirectionalSync,
	})
}

func mustUpdateOktaIntegration(ctx context.Context, t *testing.T, client oktav1.OktaServiceClient, req *oktav1.UpdateIntegrationRequest) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := client.UpdateIntegration(ctx, req)
		require.NoError(t, err)
	}, time.Second*6, time.Millisecond*30)
}

func setOktaTimeBetweenImports(t *testing.T, plugins services.Plugins, d time.Duration) {
	updateOktaPlugin(t, plugins, func(p *types.PluginV1) {
		p.Spec.GetOkta().SyncSettings.TimeBetweenImports = d.String()
	})
}

func updateOktaPlugin(t *testing.T, plugins services.Plugins, updateFn func(p *types.PluginV1)) {
	t.Helper()
	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		plugin, err := oktaplugin.Get(ctx, plugins, true)
		require.NoError(t, err)
		if plugin.Metadata.Labels == nil {
			plugin.Metadata.Labels = map[string]string{}
		}
		updateFn(plugin)
		_, err = plugins.UpdatePlugin(ctx, plugin)
		require.NoError(t, err)
	}, time.Second*10, time.Millisecond*50)
}
