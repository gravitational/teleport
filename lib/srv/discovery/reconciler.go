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

package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
)

// minBatchSize is the minimum batch size to send ServerInfos in for discovered
// instances.
const minBatchSize = 5

// serverExpirationDuration is the amount of time a Server should stay alive after being discovered.
// To be used with a jitter when creating the non-Teleport Server's Expiration.
const serverExpirationDuration = 90 * time.Minute

type serverInfoUpserter interface {
	UpsertServerInfo(ctx context.Context, si types.ServerInfo) error
}

type labelReconcilerConfig struct {
	clock       clockwork.Clock
	log         logrus.FieldLogger
	accessPoint serverInfoUpserter
}

func (c *labelReconcilerConfig) checkAndSetDefaults() error {
	if c.accessPoint == nil {
		return trace.BadParameter("missing parameter: accessPoint")
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	if c.log == nil {
		c.log = logrus.New()
	}
	return nil
}

// labelReconciler periodically reconciles the labels of discovered instances
// with the auth server.
type labelReconciler struct {
	cfg *labelReconcilerConfig

	mu                sync.Mutex
	discoveredServers map[string]types.ServerInfo
	serverInfoQueue   []types.ServerInfo
	lastBatchSize     int
	jitter            retryutils.Jitter
}

func newLabelReconciler(cfg *labelReconcilerConfig) (*labelReconciler, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &labelReconciler{
		cfg:               cfg,
		discoveredServers: make(map[string]types.ServerInfo),
		serverInfoQueue:   make([]types.ServerInfo, 0, minBatchSize),
		lastBatchSize:     minBatchSize,
		jitter:            retryutils.NewSeventhJitter(),
	}, nil
}

// getUpsertBatchSize calculates the size of batch to upsert ServerInfos in.
//
// Batches are sent once per second, and the goal is to upsert all ServerInfos
// within 15 minutes.
func getUpsertBatchSize(queueLen, lastBatchSize int) int {
	batchSize := lastBatchSize
	// Increase batch size so that all upserts can finish within 15 minutes.
	if dynamicBatchSize := (queueLen / 900) + 1; dynamicBatchSize > batchSize {
		batchSize = dynamicBatchSize
	}
	if batchSize < minBatchSize {
		batchSize = minBatchSize
	}
	if batchSize > queueLen {
		batchSize = queueLen
	}
	return batchSize
}

func (r *labelReconciler) run(ctx context.Context) {
	for ctx.Err() == nil {
		select {
		case <-r.cfg.clock.After(time.Second):
			r.mu.Lock()
			if len(r.serverInfoQueue) == 0 {
				r.mu.Unlock()
				continue
			}

			batchSize := getUpsertBatchSize(len(r.serverInfoQueue), r.lastBatchSize)
			r.lastBatchSize = batchSize
			batch := r.serverInfoQueue[:batchSize]
			r.serverInfoQueue = r.serverInfoQueue[batchSize:]

			for _, si := range batch {
				if err := r.cfg.accessPoint.UpsertServerInfo(ctx, si); err != nil {
					r.cfg.log.WithError(err).Error("Failed to upsert server info.")
					// Allow the server info to be queued again.
					delete(r.discoveredServers, si.GetName())
				}
			}
			r.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// queueServerInfos queues a list of ServerInfos to be upserted.
func (r *labelReconciler) queueServerInfos(serverInfos []types.ServerInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.cfg.clock.Now()
	for _, si := range serverInfos {
		existingInfo, ok := r.discoveredServers[si.GetName()]
		// ServerInfos should be upserted if
		//   - the instance is new
		//   - the instance's labels have changed
		//   - the existing ServerInfo will expire within 30 minutes
		if !ok ||
			!utils.StringMapsEqual(si.GetNewLabels(), existingInfo.GetNewLabels()) ||
			existingInfo.Expiry().Before(now.Add(30*time.Minute)) {

			si.SetExpiry(now.Add(r.jitter(serverExpirationDuration)))
			r.discoveredServers[si.GetName()] = si
			r.serverInfoQueue = append(r.serverInfoQueue, si)
		}
	}
}
