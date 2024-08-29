package web

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeaturesWatcher(t *testing.T) {
	clock := clockwork.NewFakeClock()
	mockedFeatures := proto.Features{
		Kubernetes:     true,
		Entitlements:   map[string]*proto.EntitlementInfo{},
		AccessRequests: &proto.AccessRequestsFeature{},
	}

	handler := &Handler{
		cfg: Config{
			LicenseWatchInterval: 100 * time.Millisecond,
			ProxyClient: &mockedPingTestProxy{
				mockedPing: func(ctx context.Context) (proto.PingResponse, error) {
					return proto.PingResponse{
						ServerFeatures: &mockedFeatures,
					}, nil
				},
			},
		},
		clock:              clock,
		clusterFeatures:    proto.Features{},
		featureWatcherStop: make(chan struct{}),
		log:                newPackageLogger(),
		logger:             slog.Default().With(teleport.ComponentKey, teleport.ComponentWeb),
	}

	// before running the watcher, features should match the value passed to the handler
	requireFeatures(t, clock, proto.Features{}, handler.GetClusterFeatures)

	go handler.startFeaturesWatcher()
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
	mockedFeatures = features
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
	mockedFeatures = features

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
	handler.stopFeaturesWatcher()
	features = proto.Features{
		Kubernetes:     !features.Kubernetes,
		App:            !features.App,
		DB:             true,
		Entitlements:   map[string]*proto.EntitlementInfo{},
		AccessRequests: &proto.AccessRequestsFeature{},
	}
	entitlements.BackfillFeatures(&features)
	mockedFeatures = features
	expected = utils.CloneProtoMsg(&features)
	// assert the handler never get these last features as the watcher is stopped
	neverFeatures(t, clock, *expected, handler.GetClusterFeatures)
}

// requireFeatures is a helper function that advances the clock, then
// calls `getFeatures` every 100ms for up to 1 second, until it
// returns the expected result (`want`).
func requireFeatures(t *testing.T, fakeClock clockwork.FakeClock, want proto.Features, getFeatures func() proto.Features) {
	t.Helper()

	// Advance the clock so the service fetch and stores features
	fakeClock.Advance(1 * time.Second)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		diff := cmp.Diff(want, getFeatures())
		if !assert.Empty(c, diff) {
			t.Logf("Feature diff (-want +got):\n%s", diff)
		}
	}, 1*time.Second, time.Millisecond*100)
}

// neverFeatures is a helper function that advances the clock, then
// calls `getFeatures` every 100ms for up to 1 second. If at some point `getFeatures`
// returns `doNotWant`, the test fails.
func neverFeatures(t *testing.T, fakeClock clockwork.FakeClock, doNotWant proto.Features, getFeatures func() proto.Features) {
	t.Helper()

	fakeClock.Advance(1 * time.Second)
	require.Never(t, func() bool {
		return cmp.Diff(doNotWant, getFeatures()) == ""
	}, 1*time.Second, time.Millisecond*100)
}
