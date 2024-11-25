package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
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

func mustRunTCTLAndGetResultAs(t *testing.T, tctl *tctlCommand, args []string, v any) {
	var output bytes.Buffer
	tctl.run(t, args, withStdout(&output))
	err := json.Unmarshal(output.Bytes(), v)
	require.NoError(t, err)
}

func mustRunTSHAndGetResultAs(t *testing.T, tsh *tshCommand, args []string, v any) {
	var output bytes.Buffer
	err := tsh.run(t, args, withStdout(&output))
	require.NoError(t, err)
	t.Log("output:", output.String())
	err = json.Unmarshal(output.Bytes(), v)
	require.NoError(t, err)
}

func mustWaitForEvent(t *testing.T, sut *common.SUT, eventType string, opts ...waitOption) {
	mustWaitForEventFrom(t, sut, eventType, time.Now(), opts...)
}

func mustWaitForEventFrom(t *testing.T, sut *common.SUT, eventType string, from time.Time, opts ...waitOption) {
	options := &waitOptions{
		timeout:   time.Second * 10,
		step:      time.Millisecond * 100,
		timePoint: time.Now(),
	}
	for _, o := range opts {
		o(options)
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		gotEvents, _, err := sut.Teleport.Process.GetAuthServer().SearchEvents(context.Background(), events.SearchEventsRequest{
			From: options.timePoint,
			To:   time.Now(),
			EventTypes: []string{
				eventType,
			},
		})
		assert.NoError(c, err)
		ok := len(gotEvents) >= 1
		assert.True(c, ok)
	}, options.timeout, options.step, "failed to wait for %s", eventType)
}

func userExistInTeleportAndIsNotLocked(t *testing.T, ctx context.Context, auth *auth.Server, oktaUser *oktaUserType) {
	_, err := auth.GetUser(ctx, oktaUser.login(), false)
	require.NoError(t, err)
	locks, err := auth.GetLocks(ctx, false, types.LockTarget{User: oktaUser.login()})
	require.NoError(t, err)
	require.Empty(t, locks)
}

type tctlCommand struct {
	DataDir  string
	Listener string
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
func (c *tctlCommand) run(t *testing.T, args []string, opts ...cmdOption) {
	options := &cmdOptions{
		stdout: os.Stdout,
	}
	for _, o := range opts {
		o(options)
	}

	yamlConfig := fmt.Sprintf(`
version: v3
teleport:
  data_dir: %s
auth_service:
  enabled: "yes"
  listen_addr: %s
`, c.DataDir, c.Listener)

	d := filepath.Join(t.TempDir(), "teleport.yaml")
	err := os.WriteFile(d, []byte(yamlConfig), 0600)
	require.NoError(t, err)

	selfExe, err := os.Executable()
	require.NoError(t, err)

	execCmd := exec.Command(selfExe,
		append([]string{fmt.Sprintf(`--config=%s`, d)}, args...)...,
	)
	execCmd.Stderr = os.Stderr
	execCmd.Stdout = options.stdout
	execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=1", testBinTCTLTestEnv))

	err = execCmd.Run()
	require.NoError(t, err)
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
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			re := regexp.MustCompile(`http://[^\s]+/[0-9a-fA-F-]{36}`)
			urls := re.FindAllString(c.out.String(), -1)
			if !assert.Len(collect, urls, 1) {
				return
			}
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
		`--auth=okta`,
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
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assignments, _, err := sut.Teleport.Process.GetAuthServer().ListOktaAssignments(context.Background(), 0, "")
		assert.NoError(collect, err)
		assert.NotEmpty(collect, assignments)
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
	mustWaitForEvent(t, sut, events.OktaUserSyncEvent, opts...)
	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, opts...)
}
