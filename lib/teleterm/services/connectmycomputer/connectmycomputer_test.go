// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connectmycomputer

import (
	"context"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
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
	accessAndIdentity := &mockAccessAndIdentity{user: oidcUser}
	certManager := &mockCertManager{}

	_, err = roleSetup.Run(ctx, accessAndIdentity, certManager, &clusters.Cluster{URI: uri.NewClusterURI("foo")})
	require.True(t, trace.IsBadParameter(err))
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

	require.Equal(t, 1, accessAndIdentity.callCounts["UpsertRole"], "expected two runs to update the role only once")
	require.Equal(t, 1, accessAndIdentity.callCounts["UpdateUser"], "expected two runs to update the user only once")
}

type mockAccessAndIdentity struct {
	user       types.User
	role       types.Role
	callCounts map[string]int
	events     *mockEvents
}

func (m *mockAccessAndIdentity) GetUser(name string, withSecrets bool) (types.User, error) {
	return m.user, nil
}

func (m *mockAccessAndIdentity) GetRole(ctx context.Context, name string) (types.Role, error) {
	if m.role != nil {
		return m.role, nil
	}
	return nil, trace.NotFound("role not found")
}

func (m *mockAccessAndIdentity) UpsertRole(ctx context.Context, role types.Role) error {
	m.callCounts["UpsertRole"]++
	m.role = role
	m.events.Fire(types.Event{
		Type:     types.OpPut,
		Resource: role,
	})
	return nil
}

func (m *mockAccessAndIdentity) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	watcher, err := m.events.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	m.events.Fire(types.Event{Type: types.OpInit})

	return watcher, nil
}

func (m *mockAccessAndIdentity) UpdateUser(ctx context.Context, user types.User) error {
	m.callCounts["UpdateUser"]++
	m.user = user
	m.events.Fire(types.Event{
		Type:     types.OpPut,
		Resource: user,
	})
	return nil
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
