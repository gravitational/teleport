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
	"bytes"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/entitlements"
)

// SetClusterFeatures sets the flags for supported and unsupported features.
// TODO(mcbattirola): make method unexported, fix tests using it to set
// test modules instead.
func (h *Handler) SetClusterFeatures(features proto.Features) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	if !bytes.Equal(h.clusterFeatures.CloudAnonymizationKey, features.CloudAnonymizationKey) {
		h.logger.InfoContext(h.cfg.Context, "Received new cloud anonymization key from server")
	}

	entitlements.BackfillFeatures(&features)
	h.clusterFeatures = features
}

// GetClusterFeatures returns flags for supported and unsupported features.
func (h *Handler) GetClusterFeatures() proto.Features {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	return h.clusterFeatures
}

// startFeatureWatcher periodically pings the auth server and updates `clusterFeatures`.
// Must be called only once per `handler`, otherwise it may close an already closed channel
// which will cause a panic.
// The watcher doesn't ping the auth server immediately upon start because features are
// already set by the config object in `NewHandler`.
func (h *Handler) startFeatureWatcher() {
	ctx := h.cfg.Context
	ticker := h.clock.NewTicker(h.cfg.FeatureWatchInterval)
	h.logger.InfoContext(ctx, "Proxy handler features watcher has started", "interval", h.cfg.FeatureWatchInterval)

	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			h.logger.InfoContext(ctx, "Pinging auth server for features")
			pingResponse, err := h.GetProxyClient().Ping(ctx)
			if err != nil {
				h.logger.ErrorContext(ctx, "Auth server ping failed", "error", err)
				continue
			}

			h.SetClusterFeatures(*pingResponse.ServerFeatures)
			h.logger.InfoContext(ctx, "Done updating proxy features", "features", pingResponse.ServerFeatures)
		case <-ctx.Done():
			h.logger.InfoContext(ctx, "Feature service has stopped")
			return
		}
	}
}
