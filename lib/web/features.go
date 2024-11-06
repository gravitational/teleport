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

// SetClusterFeatures sets the flags for supported and unsupported features
func (h *Handler) SetClusterFeatures(features proto.Features) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	if !bytes.Equal(h.clusterFeatures.CloudAnonymizationKey, features.CloudAnonymizationKey) {
		h.log.Info("Received new cloud anonymization key from server")
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
func (h *Handler) startFeatureWatcher() {
	ticker := h.clock.NewTicker(h.cfg.FeatureWatchInterval)
	h.log.WithField("interval", h.cfg.FeatureWatchInterval).Info("Proxy handler features watcher has started")
	ctx := h.cfg.Context

	// close ready channel to signal it started the main loop
	if h.featureWatcherReady != nil {
		close(h.featureWatcherReady)
	}

	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			h.log.Info("Pinging auth server for features")
			pingResponse, err := h.GetProxyClient().Ping(ctx)
			if err != nil {
				h.log.WithError(err).Error("Auth server ping failed")
				continue
			}

			h.SetClusterFeatures(*pingResponse.ServerFeatures)
			h.log.WithField("features", pingResponse.ServerFeatures).Info("Done updating proxy features")
		case <-ctx.Done():
			h.log.Info("Feature service has stopped")
			return
		}
	}
}
