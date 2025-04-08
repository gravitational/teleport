package auth

import (
	"context"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type fakeControlStream struct {
	client.UpstreamInventoryControlStream
}

func (f fakeControlStream) Close() error {
	return nil
}

func (f fakeControlStream) Recv() <-chan proto.UpstreamInventoryMessage {
	return make(chan proto.UpstreamInventoryMessage)
}

func (f fakeControlStream) Done() <-chan struct{} {
	return make(chan struct{})
}

type fakeServer struct {
	version       string
	updateGroup   string
	delay         time.Duration
	roles         types.SystemRoles
	updaterStatus proto.UpdaterStatus
}

func TestServer_generateAgentVersionReport(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	twoMinutesAgo := now.Add(-time.Minute * 2)
	// agentRole are typicial roles an agent can have
	agentRoles := types.SystemRoles{types.RoleNode, types.RoleApp}
	updaterOK := proto.UpdaterStatus_UpdaterStatusOK

	tests := []struct {
		name     string
		fixtures []fakeServer
		expected instanceReport
	}{
		{
			name:     "no servers",
			expected: instanceReport{timestamp: now, data: make(map[string]instanceGroupReport)},
		},
		{
			name: "no group, same version",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: agentRoles},
				{version: "1.2.3", roles: agentRoles},
				{version: "1.2.3", roles: agentRoles},
				{version: "1.2.3", roles: agentRoles},
			},
			expected: instanceReport{timestamp: now, data: map[string]instanceGroupReport{
				"": {"1.2.3": {count: 4}},
			}},
		},
		{
			name: "control plane servers are ignored",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: types.SystemRoles{types.RoleKube, types.RoleAuth}},
				{version: "1.2.3", roles: types.SystemRoles{types.RoleApp, types.RoleProxy}},
				{version: "1.2.3", roles: types.SystemRoles{types.RoleApp, types.RoleKube}},
			},
			expected: instanceReport{timestamp: now, data: map[string]instanceGroupReport{
				"": {"1.2.3": {count: 1}},
			}},
		},

		{
			name: "disabled or pinned updaters are ignored",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.3", roles: agentRoles, updaterStatus: proto.UpdaterStatus_UpdaterStatusPinned},
				{version: "1.2.3", roles: agentRoles, updaterStatus: proto.UpdaterStatus_UpdaterStatusDisabled},
			},
			expected: instanceReport{timestamp: now, data: map[string]instanceGroupReport{
				"": {"1.2.3": {count: 1}},
			}},
		},
		{
			name: "too recent servers are ignored",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.3", delay: 90 * time.Second, roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK},
			},
			expected: instanceReport{timestamp: now, data: map[string]instanceGroupReport{
				"": {"1.2.3": {count: 1}},
			}},
		},
		{
			name: "multiple versions and groups",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.4", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.5", updateGroup: "prod", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.5", updateGroup: "prod", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.5", updateGroup: "dev", roles: agentRoles, updaterStatus: updaterOK},
			},
			expected: instanceReport{timestamp: now, data: map[string]instanceGroupReport{
				"": {
					"1.2.3": {count: 1},
					"1.2.4": {count: 1},
				},
				"dev": {
					"1.2.5": {count: 1},
				},
				"prod": {
					"1.2.5": {count: 2},
				},
			},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clockwork.NewFakeClockAt(twoMinutesAgo)
			auth := &Server{}
			controller := inventory.NewController(auth, nil, inventory.WithClock(clock))
			for _, fixture := range tt.fixtures {
				clock.Advance(fixture.delay)
				controller.RegisterControlStream(fakeControlStream{}, proto.UpstreamInventoryHello{
					Services:      fixture.roles,
					ServerID:      uuid.New().String(),
					Version:       fixture.version,
					UpdateGroup:   fixture.updateGroup,
					UpdaterStatus: fixture.updaterStatus,
				})
			}
			auth.inventory = controller
			auth.clock = clockwork.NewFakeClockAt(now)

			require.Equal(t, tt.expected, auth.generateAgentVersionReport(ctx))
		})
	}
}
