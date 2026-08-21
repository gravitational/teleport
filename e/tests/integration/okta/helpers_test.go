package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/e/tests/common/tctl"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/itertools/stream"
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

func waitForPerUserOktaAssignments(t *testing.T, watcher types.Watcher, count int) {
	t.Helper()
	seen := make(map[string]struct{})
	waitForResource(t, watcher, func(assignment *types.OktaAssignmentV1) bool {
		seen[assignment.GetUser()] = struct{}{}
		return len(seen) >= count
	})
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

func mustCreateIntegration(t *testing.T, sut *common.SUT, oktaAuthClient oktav1.OktaServiceClient, settings createIntegrationSettings) {
	t.Helper()
	ctx := t.Context()

	_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ApiCredentials:          settings.apiCredentials,
		ReuseConnector:          settings.reuseConnector,
		EnableUserSync:          settings.enableUserSync,
		EnableAppGroupSync:      settings.enableAppGroupSync,
		EnableAccessListSync:    settings.enableAccessListSync,
		EnableBidirectionalSync: settings.enableBidirectionalSync,
	}.Build())
	require.NoError(t, err)
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})
}

func mustUpdateIntegration(t *testing.T, sut *common.SUT, oktaAuthClient oktav1.OktaServiceClient, settings integrationSettings) {
	t.Helper()
	ctx := t.Context()

	mustUpdateOktaIntegration(ctx, t, oktaAuthClient, oktav1.UpdateIntegrationRequest_builder{
		EnableUserSync:          settings.enableUserSync,
		EnableAppGroupSync:      settings.enableAppGroupSync,
		EnableAccessListSync:    settings.enableAccessListSync,
		EnableBidirectionalSync: settings.enableBidirectionalSync,
	}.Build())
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})
}

func mustUpdateOktaIntegration(ctx context.Context, t *testing.T, client oktav1.OktaServiceClient, req *oktav1.UpdateIntegrationRequest) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := client.UpdateIntegration(ctx, req)
		require.NoError(t, err)
	}, 30*time.Second, time.Millisecond*30)
}

type delays struct {
	timeBetweenImports                time.Duration
	timeBetweenAssignmentProcessLoops time.Duration
	targetProcessingBackoffStep       time.Duration
	targetProcessingBackoffMax        time.Duration
}

func updateOktaDelays(t *testing.T, sut *common.SUT, delays delays) {
	t.Helper()
	authServer := sut.Teleport.Process.GetAuthServer().Services
	updateOktaPlugin(t, authServer, func(p *types.PluginV1) {
		p.Spec.GetOkta().SyncSettings.TimeBetweenImports = delays.timeBetweenImports.String()
		p.Spec.GetOkta().SyncSettings.TimeBetweenAssignmentProcessLoops = delays.timeBetweenAssignmentProcessLoops.String()
		p.Spec.GetOkta().SyncSettings.TargetProcessingBackoffStep = delays.targetProcessingBackoffStep.String()
		p.Spec.GetOkta().SyncSettings.TargetProcessingBackoffMax = delays.targetProcessingBackoffMax.String()
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

func waitForResource[T types.Resource](t *testing.T, watcher types.Watcher, fn func(T) bool) T {
	t.Helper()
	return common.WaitForPutEvent(t, watcher, fn)
}

func waitForResourceCount[T types.Resource](t *testing.T, watcher types.Watcher, expectedCnt int, fn func(T) bool) []T {
	t.Helper()
	seen := make(map[string]T, expectedCnt)
	waitForResource(t, watcher, func(r T) bool {
		if !fn(r) {
			delete(seen, r.GetName())
			return false
		}
		seen[r.GetName()] = r
		return len(seen) == expectedCnt
	})
	return slices.Collect(maps.Values(seen))
}

func waitForResourceDeletion[T types.Resource](t *testing.T, watcher types.Watcher, fn func(T) bool) T {
	t.Helper()
	return common.WaitForDeleteEvent(t, watcher, fn)
}

func mustListAccessListMembers(t *testing.T, sut *common.SUT, accessListName string) []*accesslist.AccessListMember {
	t.Helper()
	members, err := stream.Collect(clientutils.Resources(t.Context(),
		func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
			return sut.Teleport.Process.GetAuthServer().ListAccessListMembers(ctx, accessListName, pageSize, pageToken)
		},
	))
	require.NoError(t, err)
	return members
}

func mustUpsertAccessListMember(t *testing.T, sut *common.SUT, accessList *accesslist.AccessList, user types.User) {
	t.Helper()
	ctx := t.Context()

	authServer := sut.Teleport.Process.GetAuthServer()

	m := mustCreateMember(t, accessList.GetName(), user.GetName(), accesslist.MembershipKindUser)

	_, err := authServer.UpsertAccessListMember(ctx, m)
	require.NoError(t, err)
}

func mustDeleteAccessListMember(t *testing.T, sut *common.SUT, accessList *accesslist.AccessList, user types.User) {
	t.Helper()
	ctx := t.Context()

	authServer := sut.Teleport.Process.GetAuthServer()

	err := authServer.DeleteAccessListMember(ctx, accessList.GetName(), user.GetName())
	require.NoError(t, err)
}

func mustGetEventsFrom(t require.TestingT, sut *common.SUT, from time.Time, eventTypes ...string) []apievents.AuditEvent {
	callHelper(t)
	ctx := getContextOrBackground(t)

	services := sut.Teleport.Process.GetAuthServer().Services

	listFn := func(ctx context.Context, limit int, pageToken string) ([]apievents.AuditEvent, string, error) {
		return services.SearchEvents(ctx, events.SearchEventsRequest{
			From:       from,
			To:         time.Now(),
			Limit:      limit,
			EventTypes: eventTypes,
			StartKey:   pageToken,
		})
	}

	var res []apievents.AuditEvent
	for ev, err := range clientutils.Resources(ctx, listFn) {
		require.NoError(t, err)
		res = append(res, ev)
	}

	return res
}

func mustGetAccessLists(t *testing.T, sut *common.SUT) []*accesslist.AccessList {
	t.Helper()
	ctx := t.Context()

	services := sut.Teleport.Process.GetAuthServer().Services

	var res []*accesslist.AccessList
	for al, err := range clientutils.Resources(ctx, services.ListAccessLists) {
		require.NoError(t, err)
		res = append(res, al)
	}

	return res
}

func mustGetOktaUsers(t *testing.T, sut *common.SUT) []types.User {
	t.Helper()
	ctx := t.Context()

	services := sut.Teleport.Process.GetAuthServer().Services

	users, err := services.GetUsers(ctx, false)
	require.NoError(t, err)

	return slices.DeleteFunc(users, func(u types.User) bool {
		return u.Origin() != types.OriginOkta
	})
}

// mustGetAssignmentForUser requires exactly one assignment to be found for the user and returns it.
func mustGetAssignmentForUser(t require.TestingT, sut *common.SUT, user string) types.OktaAssignment {
	callHelper(t)
	ctx := getContextOrBackground(t)

	services := sut.Teleport.Process.GetAuthServer().Services

	var assignments []types.OktaAssignment
	for assignment, err := range clientutils.Resources(ctx, services.Okta.ListOktaAssignments) {
		require.NoError(t, err)
		if assignment.GetUser() == user {
			assignments = append(assignments, assignment)
		}
	}
	require.Len(t, assignments, 1)

	return assignments[0]
}

func callHelper(t require.TestingT) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
}

func getContextOrBackground(t require.TestingT) context.Context {
	if c, ok := t.(interface{ Context() context.Context }); ok {
		return c.Context()
	}
	return context.Background()
}

func getResourceNames[T types.Resource](rs []T) []string {
	names := make([]string, len(rs))
	for i, r := range rs {
		names[i] = r.GetName()
	}
	return names
}

func assignmentHasTarget(a types.OktaAssignment, typ, id string) bool {
	for _, t := range a.GetTargets() {
		if t.GetTargetType() == typ && t.GetID() == id {
			return true
		}
	}
	return false
}
