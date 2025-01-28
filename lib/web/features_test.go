/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package web

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// mockedPingTestProxy is a test proxy with a mocked Ping method
// that returns the internal features
type mockedFeatureGetter struct {
	authclient.ClientI
	features proto.Features
}

func (m *mockedFeatureGetter) Ping(ctx context.Context) (proto.PingResponse, error) {
	return proto.PingResponse{
		ServerFeatures: utils.CloneProtoMsg(&m.features),
	}, nil
}

func (m *mockedFeatureGetter) setFeatures(f proto.Features) {
	m.features = f
}

func TestFeaturesWatcher(t *testing.T) {
	clock := clockwork.NewFakeClock()

	mockClient := &mockedFeatureGetter{features: proto.Features{
		Kubernetes:     true,
		Entitlements:   map[string]*proto.EntitlementInfo{},
		AccessRequests: &proto.AccessRequestsFeature{},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	handler := &Handler{
		cfg: Config{
			FeatureWatchInterval: 100 * time.Millisecond,
			ProxyClient:          mockClient,
			Context:              ctx,
		},
		clock:           clock,
		clusterFeatures: proto.Features{},
		logger:          slog.Default().With(teleport.ComponentKey, teleport.ComponentWeb),
	}

	// before running the watcher, features should match the value passed to the handler
	requireFeatures(t, clock, proto.Features{}, handler.GetClusterFeatures)

	go handler.startFeatureWatcher()
	clock.BlockUntil(1)

	// after starting the watcher, handler.GetClusterFeatures should return
	// values matching the client's response
	features := proto.Features{
		Kubernetes:     true,
		Entitlements:   map[string]*proto.EntitlementInfo{},
		AccessRequests: &proto.AccessRequestsFeature{},
	}
	entitlements.BackfillFeatures(&features)
	expected := utils.CloneProtoMsg(&features)
	requireFeatures(t, clock, *expected, handler.GetClusterFeatures)

	// update values once again and check if the features are properly updated
	features = proto.Features{
		Kubernetes:     false,
		Entitlements:   map[string]*proto.EntitlementInfo{},
		AccessRequests: &proto.AccessRequestsFeature{},
	}
	entitlements.BackfillFeatures(&features)
	mockClient.setFeatures(features)
	expected = utils.CloneProtoMsg(&features)
	requireFeatures(t, clock, *expected, handler.GetClusterFeatures)

	// test updating entitlements
	features = proto.Features{
		Kubernetes: true,
		Entitlements: map[string]*proto.EntitlementInfo{
			string(entitlements.ExternalAuditStorage):   {Enabled: true},
			string(entitlements.AccessLists):            {Enabled: true},
			string(entitlements.AccessMonitoring):       {Enabled: true},
			string(entitlements.App):                    {Enabled: true},
			string(entitlements.CloudAuditLogRetention): {Enabled: true},
		},
		AccessRequests: &proto.AccessRequestsFeature{},
	}
	entitlements.BackfillFeatures(&features)
	mockClient.setFeatures(features)

	expected = &proto.Features{
		Kubernetes: true,
		Entitlements: map[string]*proto.EntitlementInfo{
			string(entitlements.ExternalAuditStorage):   {Enabled: true},
			string(entitlements.AccessLists):            {Enabled: true},
			string(entitlements.AccessMonitoring):       {Enabled: true},
			string(entitlements.App):                    {Enabled: true},
			string(entitlements.CloudAuditLogRetention): {Enabled: true},
		},
		AccessRequests: &proto.AccessRequestsFeature{},
	}
	entitlements.BackfillFeatures(expected)
	requireFeatures(t, clock, *expected, handler.GetClusterFeatures)

	// stop watcher and ensure it stops updating features
	cancel()
	features = proto.Features{
		Kubernetes:     !features.Kubernetes,
		App:            !features.App,
		DB:             true,
		Entitlements:   map[string]*proto.EntitlementInfo{},
		AccessRequests: &proto.AccessRequestsFeature{},
	}
	entitlements.BackfillFeatures(&features)
	mockClient.setFeatures(features)
	notExpected := utils.CloneProtoMsg(&features)
	// assert the handler never get these last features as the watcher is stopped
	neverFeatures(t, clock, *notExpected, handler.GetClusterFeatures)
}

// requireFeatures is a helper function that advances the clock, then
// calls `getFeatures` every 100ms for up to 1 second, until it
// returns the expected result (`want`).
func requireFeatures(t *testing.T, fakeClock *clockwork.FakeClock, want proto.Features, getFeatures func() proto.Features) {
	t.Helper()

	// Advance the clock so the service fetch and stores features
	fakeClock.Advance(1 * time.Second)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		diff := cmp.Diff(want, getFeatures())
		assert.Empty(t, diff)
	}, 5*time.Second, time.Millisecond*100)
}

// neverFeatures is a helper function that advances the clock, then
// calls `getFeatures` every 100ms for up to 1 second. If at some point `getFeatures`
// returns `doNotWant`, the test fails.
func neverFeatures(t *testing.T, fakeClock *clockwork.FakeClock, doNotWant proto.Features, getFeatures func() proto.Features) {
	t.Helper()

	fakeClock.Advance(1 * time.Second)
	require.Never(t, func() bool {
		return cmp.Diff(doNotWant, getFeatures()) == ""
	}, 1*time.Second, time.Millisecond*100)
}
