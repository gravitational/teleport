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

package connectmycomputer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

func TestRoleSetupRun_WithNonLocalUser(t *testing.T) {
	roleSetup, err := NewRoleSetup(&RoleSetupConfig{})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	oidcUser, err := types.NewUser("alice")
	require.NoError(t, err)
	oidcUser.SetCreatedBy(types.CreatedBy{
		Connector: &types.ConnectorRef{Type: "oidc", ID: "google"},
	})
	accessAndIdentity := &mockAccessAndIdentity{
		user:       oidcUser,
		callCounts: make(map[string]int),
		events:     &mockEvents{},
	}
	certManager := &mockCertManager{}

	_, err = roleSetup.Run(ctx, accessAndIdentity, certManager, &clusters.Cluster{URI: uri.NewClusterURI("foo")})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected the error to be BadParameter")
}

// During development, I already managed to introduce a bug in a conditional which resulted in a
// resource being updated on every run of RoleSetup.
// The integration tests won't catch that since they worry about the end result only.
func TestRoleSetupRun_Idempotency(t *testing.T) {
	roleSetup, err := NewRoleSetup(&RoleSetupConfig{})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	user, err := types.NewUser("alice")
	require.NoError(t, err)
	events := &mockEvents{}
	accessAndIdentity := &mockAccessAndIdentity{
		user:       user,
		callCounts: make(map[string]int),
		events:     events,
	}
	certManager := &mockCertManager{}

	_, err = roleSetup.Run(ctx, accessAndIdentity, certManager, &clusters.Cluster{URI: uri.NewClusterURI("foo")})
	require.NoError(t, err)

	_, err = roleSetup.Run(ctx, accessAndIdentity, certManager, &clusters.Cluster{URI: uri.NewClusterURI("foo")})
	require.NoError(t, err)

	require.Equal(t, 1, accessAndIdentity.callCounts["CreateRole"], "expected two runs to create the role only once")
	require.Equal(t, 0, accessAndIdentity.callCounts["UpdateRole"], "expected two runs to not update the role")
	require.Equal(t, 1, accessAndIdentity.callCounts["UpdateUser"], "expected two runs to update the user only once")
}

func TestRoleSetupRun_RoleErrors(t *testing.T) {
	existingRole, err := types.NewRole("connect-my-computer-alice", types.RoleSpecV6{})
	require.NoError(t, err)

	bogusErr := errors.New("something went wrong")

	tests := []struct {
		name          string
		existingRole  types.Role
		createRoleErr error
		updateRoleErr error
		wantErr       error
	}{
		{
			name:          "creating role fails",
			createRoleErr: bogusErr,
			wantErr:       bogusErr,
		},
		{
			name:          "updating role fails",
			existingRole:  existingRole,
			updateRoleErr: bogusErr,
			wantErr:       bogusErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			user, err := types.NewUser("alice")
			require.NoError(t, err)

			events := &mockEvents{}
			certManager := &mockCertManager{}
			accessAndIdentity := &mockAccessAndIdentity{
				user:          user,
				callCounts:    make(map[string]int),
				events:        events,
				role:          tt.existingRole,
				createRoleErr: tt.createRoleErr,
				updateRoleErr: tt.updateRoleErr,
			}

			roleSetup, err := NewRoleSetup(&RoleSetupConfig{})
			require.NoError(t, err)

			_, err = roleSetup.Run(ctx, accessAndIdentity, certManager, &clusters.Cluster{URI: uri.NewClusterURI("foo")})
			require.Error(t, err)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

const nodejoinWaitTestTimeout = 10 * time.Second

func TestNodeJoinWaitRun_WaitsForHostUUIDFileToBeCreatedAndFetchesNodeFromCluster(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), nodejoinWaitTestTimeout)
	t.Cleanup(cancel)

	cluster := &clusters.Cluster{URI: uri.NewClusterURI("foo"), ProfileName: "foo"}
	events := &mockEvents{}
	node, err := types.NewServer("1234", types.KindNode, types.ServerSpecV2{
		CmdLabels: types.LabelsToV2(map[string]types.CommandLabel{
			defaults.HostnameLabel: &types.CommandLabelV2{Result: ""},
		}),
	})
	require.NoError(t, err)
	accessAndIdentity := &mockAccessAndIdentity{
		callCounts: make(map[string]int),
		events:     events,
		node:       node,
	}

	nodeJoinWait, err := NewNodeJoinWait(&NodeJoinWaitConfig{AgentsDir: t.TempDir()})
	require.NoError(t, err)

	runErr := make(chan error)
	serverC := make(chan clusters.Server)

	go func() {
		server, err := nodeJoinWait.Run(ctx, accessAndIdentity, cluster)
		runErr <- err
		serverC <- server
	}()

	// Make sure NodeJoinWait.Run doesn't see the file on the first tick.
	time.Sleep(10 * time.Millisecond)

	// Create the UUID file while NodeJoinWait.Run is executed in a separate goroutine to verify that
	// it continuously attempts to read the host UUID file, rather than reading it only once.
	mustMakeHostUUIDFile(t, nodeJoinWait.cfg.AgentsDir, cluster.ProfileName)

	// Verify that NodeJoinWait.Run used GetNode and not a watcher to fetch the node.
	require.NoError(t, <-runErr)
	server := <-serverC
	require.Equal(t, node.GetName(), server.GetName())

	// Verify that the empty hostname label gets filled out.
	hostname, err := os.Hostname()
	require.NoError(t, err)
	require.Contains(t, server.GetCmdLabels(), defaults.HostnameLabel)
	require.Equal(t, hostname, server.GetCmdLabels()[defaults.HostnameLabel].GetResult())
}

func TestNodeJoinWaitRun_WatchesForOpPutIfNodeWasNotFound(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), nodejoinWaitTestTimeout)
	t.Cleanup(cancel)

	cluster := &clusters.Cluster{URI: uri.NewClusterURI("foo"), ProfileName: "foo"}
	events := &mockEvents{}
	accessAndIdentity := &mockAccessAndIdentity{
		callCounts: make(map[string]int),
		events:     events,
		// Setting to true because we manually fire OpPut from test body.
		requireManualOpInitFire: true,
	}

	nodeJoinWait, err := NewNodeJoinWait(&NodeJoinWaitConfig{AgentsDir: t.TempDir()})
	require.NoError(t, err)

	hostUUID := mustMakeHostUUIDFile(t, nodeJoinWait.cfg.AgentsDir, cluster.ProfileName)
	eventServer, err := types.NewServer(hostUUID, types.KindNode, types.ServerSpecV2{
		CmdLabels: types.LabelsToV2(map[string]types.CommandLabel{
			defaults.HostnameLabel: &types.CommandLabelV2{Result: ""},
		}),
	})
	require.NoError(t, err)
	bogusEventServer, err := types.NewServer("1234", types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)

	runErr := make(chan error)
	serverC := make(chan clusters.Server)

	go func() {
		server, err := nodeJoinWait.Run(ctx, accessAndIdentity, cluster)
		runErr <- err
		serverC <- server
	}()

	err = accessAndIdentity.events.WaitSomeWatchers(ctx)
	require.NoError(t, err)
	accessAndIdentity.events.Fire(types.Event{Type: types.OpInit})

	// Fire an event with another node first to verify that NodeJoinWait does the comparison correctly.
	accessAndIdentity.events.Fire(types.Event{
		Type:     types.OpPut,
		Resource: bogusEventServer,
	})
	accessAndIdentity.events.Fire(types.Event{
		Type:     types.OpPut,
		Resource: eventServer,
	})

	// Verify that NodeJoinWait.Run returns as soon as it receives an event with a matching server.
	require.NoError(t, <-runErr)
	server := <-serverC
	require.Equal(t, eventServer.GetName(), server.GetName())

	// Verify that the empty hostname label gets filled out.
	hostname, err := os.Hostname()
	require.NoError(t, err)
	require.Contains(t, server.GetCmdLabels(), defaults.HostnameLabel)
	require.Equal(t, hostname, server.GetCmdLabels()[defaults.HostnameLabel].GetResult())
}

func TestNodeJoinWaitRun_ReturnsEarlyIfGetNodeReturnsErrorOtherThanNotFound(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), nodejoinWaitTestTimeout)
	t.Cleanup(cancel)

	cluster := &clusters.Cluster{URI: uri.NewClusterURI("foo"), ProfileName: "foo"}
	events := &mockEvents{}
	nodeErr := trace.Errorf("something went wrong")
	accessAndIdentity := &mockAccessAndIdentity{
		callCounts: make(map[string]int),
		events:     events,
		nodeErr:    nodeErr,
	}

	nodeJoinWait, err := NewNodeJoinWait(&NodeJoinWaitConfig{AgentsDir: t.TempDir()})
	require.NoError(t, err)

	mustMakeHostUUIDFile(t, nodeJoinWait.cfg.AgentsDir, cluster.ProfileName)

	_, err = nodeJoinWait.Run(ctx, accessAndIdentity, cluster)
	require.Equal(t, nodeErr, err)
}

type mockAccessAndIdentity struct {
	user       types.User
	role       types.Role
	callCounts map[string]int
	events     *mockEvents
	// requireManualOpInitFire makes mockAccessAndIdentity.NewWatcher skip firing OpInit.
	//
	// In regular tests where this field is false, the code under tests calls
	// mockAccessAndIdentity.NewWatcher (which fires OpInit), waits for OpInit, and then calls another
	// method on mockAccessAndIdentity which fires an event.
	//
	// In tests where events such as OpPut are triggered directly from the test body and not as a
	// result of the code under tests calling methods on mockAccessAndIdentity, setting it to true
	// allows manually firing OpInit first before firing other events. This ensures that the first
	// event that watchers observe is OpInit.
	//
	requireManualOpInitFire bool
	node                    types.Server
	nodeErr                 error
	createRoleErr           error
	updateRoleErr           error
}

func (m *mockAccessAndIdentity) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	return m.user, nil
}

func (m *mockAccessAndIdentity) GetRole(ctx context.Context, name string) (types.Role, error) {
	if m.role != nil {
		return m.role, nil
	}
	return nil, trace.NotFound("role not found")
}

func (m *mockAccessAndIdentity) CreateRole(ctx context.Context, role types.Role) (types.Role, error) {
	m.callCounts["CreateRole"]++

	if m.createRoleErr != nil {
		return nil, m.createRoleErr
	}

	m.role = role
	m.events.Fire(types.Event{
		Type:     types.OpPut,
		Resource: role,
	})
	return role, nil
}

func (m *mockAccessAndIdentity) UpdateRole(ctx context.Context, role types.Role) (types.Role, error) {
	m.callCounts["UpdateRole"]++

	if m.updateRoleErr != nil {
		return nil, m.updateRoleErr
	}

	m.role = role
	m.events.Fire(types.Event{
		Type:     types.OpPut,
		Resource: role,
	})
	return role, nil
}

func (m *mockAccessAndIdentity) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	watcher, err := m.events.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !m.requireManualOpInitFire {
		m.events.Fire(types.Event{Type: types.OpInit})
	}

	return watcher, nil
}

func (m *mockAccessAndIdentity) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	m.callCounts["UpdateUser"]++
	m.user = user
	m.events.Fire(types.Event{
		Type:     types.OpPut,
		Resource: user,
	})
	return user, nil
}

func (m *mockAccessAndIdentity) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	if m.nodeErr != nil {
		return nil, m.nodeErr
	}

	if m.node != nil {
		return m.node, nil
	}
	return nil, trace.NotFound("node not found")
}

type mockCertManager struct{}

func (m *mockCertManager) ReissueUserCerts(context.Context, client.CertCachePolicy, client.ReissueParams) error {
	return nil
}

// mockEvents enables sending out events to watchers from within a test or other mocks.
// The implementation is copied from integrations/lib/watcherjob/helpers_test.go.
type mockEvents struct {
	sync.Mutex
	channels []chan<- types.Event
}

// NewWatcher creates a new watcher.
func (e *mockEvents) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	events := make(chan types.Event, 1000)
	e.Lock()
	e.channels = append(e.channels, events)
	e.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	return mockWatcher{events: events, ctx: ctx, cancel: cancel}, ctx.Err()
}

// Fire emits a watcher events for all the subscribers to consume.
func (e *mockEvents) Fire(event types.Event) {
	e.Lock()
	channels := e.channels
	e.Unlock()
	for _, events := range channels {
		events <- event
	}
}

// WaitSomeWatchers blocks until either some watcher is subscribed or context is done.
func (e *mockEvents) WaitSomeWatchers(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.Lock()
			n := len(e.channels)
			e.Unlock()
			if n > 0 {
				return nil
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

// mockWatcher is copied from integrations/lib/watcherjob/helpers_test.go.
type mockWatcher struct {
	events <-chan types.Event
	ctx    context.Context
	cancel context.CancelFunc
}

// Events returns a stream of events.
func (w mockWatcher) Events() <-chan types.Event {
	return w.events
}

// Done returns a completion channel.
func (w mockWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

// Close sends a termination signal to watcher.
func (w mockWatcher) Close() error {
	w.cancel()
	return nil
}

// Error returns a watcher error.
func (w mockWatcher) Error() error {
	return trace.Wrap(w.ctx.Err())
}

func mustMakeHostUUIDFile(t *testing.T, agentsDir string, profileName string) string {
	dataDir := filepath.Join(agentsDir, profileName, "data")

	agentsDirStat, err := os.Stat(agentsDir)
	require.NoError(t, err)

	err = os.MkdirAll(dataDir, agentsDirStat.Mode())
	require.NoError(t, err)

	hostUUID, err := hostid.ReadOrCreateFile(dataDir)
	require.NoError(t, err)

	return hostUUID
}

func TestNodeNameGet(t *testing.T) {
	t.Parallel()

	cluster := &clusters.Cluster{URI: uri.NewClusterURI("foo"), ProfileName: "foo"}
	nodeName, err := NewNodeName(&NodeNameConfig{AgentsDir: t.TempDir()})
	require.NoError(t, err)
	hostUUID := mustMakeHostUUIDFile(t, nodeName.cfg.AgentsDir, cluster.ProfileName)

	readUUID, err := nodeName.Get(cluster)

	require.NoError(t, err)
	require.Equal(t, readUUID, hostUUID)
}
