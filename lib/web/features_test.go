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
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// mockedFeatureGetter is a test proxy with a mocked Ping method
// that returns the internal features
type mockedFeatureGetter struct {
	authclient.ClientI

	mu       sync.Mutex
	features proto.Features
}

func (m *mockedFeatureGetter) Ping(ctx context.Context) (proto.PingResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return proto.PingResponse{
		ServerFeatures: utils.CloneProtoMsg(&m.features),
	}, nil
}

func (m *mockedFeatureGetter) setFeatures(f proto.Features) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.features = f
}

func TestFeaturesWatcher(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &mockedFeatureGetter{features: proto.Features{
			Kubernetes:     true,
			Entitlements:   map[string]*proto.EntitlementInfo{},
			AccessRequests: &proto.AccessRequestsFeature{},
		}}

		ctx, cancel := context.WithCancel(t.Context())

		handler := &Handler{
			cfg: Config{
				FeatureWatchInterval: 100 * time.Millisecond,
				ProxyClient:          mockClient,
				Context:              ctx,
			},
			clock:           clockwork.NewRealClock(),
			clusterFeatures: proto.Features{},
			logger:          slog.Default().With(teleport.ComponentKey, teleport.ComponentWeb),
		}

		go handler.startFeatureWatcher()
		synctest.Wait()

		// before running the watcher, features should match the value passed to the handler
		require.Equal(t, proto.Features{}, handler.GetClusterFeatures())

		// advance and wait. once this returns the first feature update will have completed
		// and the updater will be blocked prior to the second update.
		time.Sleep(handler.cfg.FeatureWatchInterval)
		synctest.Wait()

		// after starting the watcher, handler.GetClusterFeatures should return
		// values matching the client's response
		features := proto.Features{
			Kubernetes:     true,
			Entitlements:   map[string]*proto.EntitlementInfo{},
			AccessRequests: &proto.AccessRequestsFeature{},
		}
		entitlements.BackfillFeatures(&features)
		expected := utils.CloneProtoMsg(&features)
		require.Equal(t, *expected, handler.GetClusterFeatures())

		// update values once again and check if the features are properly updated
		features = proto.Features{
			Kubernetes:     false,
			Entitlements:   map[string]*proto.EntitlementInfo{},
			AccessRequests: &proto.AccessRequestsFeature{},
		}
		entitlements.BackfillFeatures(&features)
		mockClient.setFeatures(features)

		time.Sleep(handler.cfg.FeatureWatchInterval)
		synctest.Wait()

		expected = utils.CloneProtoMsg(&features)
		require.Equal(t, *expected, handler.GetClusterFeatures())

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

		time.Sleep(handler.cfg.FeatureWatchInterval)
		synctest.Wait()

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
		require.Equal(t, *expected, handler.GetClusterFeatures())

		// stop watcher and ensure it stops updating features
		cancel()
		synctest.Wait()

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
		require.NotEqual(t, *notExpected, handler.GetClusterFeatures())
	})
}
