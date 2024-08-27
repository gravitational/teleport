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

// Package web implements web proxy handler that provides
// web interface to view and connect to teleport nodes
package web

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/entitlements"
)

func (h *Handler) SetClusterFeatures(features proto.Features) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	entitlements.SupportEntitlementsCompatibility(&features)
	h.clusterFeatures = features
}

func (h *Handler) GetClusterFeatures() proto.Features {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	return h.clusterFeatures
}

func (h *Handler) startFeaturesWatcher() {
	ticker := h.clock.NewTicker(h.cfg.LicenseWatchInterval)
	h.log.WithField("interval", h.cfg.LicenseWatchInterval).Info("Proxy handler features watcher has started")
	ctx := context.Background()

	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			h.log.Info("Pinging auth server for features")
			f, err := h.GetProxyClient().Ping(ctx)
			if err != nil {
				h.log.WithError(err).Error("Failed fetching features")
				continue
			}

			h.SetClusterFeatures(*f.ServerFeatures)
			h.log.WithField("features", f.ServerFeatures).Infof("Done updating proxy features: %+v", f)
		case <-h.featureWatcherStop:
			h.log.Info("Feature service has stopped")
			return
		}
	}
}

func (h *Handler) stopFeaturesWatcher() {
	close(h.featureWatcherStop)
}
