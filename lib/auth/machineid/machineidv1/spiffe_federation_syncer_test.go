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

package machineidv1

import (
	"context"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSPIFFEFederationSyncer_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := utils.NewSlogLoggerForTests()
	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	spiffeFederationStore, err := local.NewSPIFFEFederationService(backend)
	require.NoError(t, err)
	eventsSvc := local.NewEventsService(backend)

	syncer, err := NewSPIFFEFederationSyncer(SPIFFEFederationSyncerConfig{
		Backend:       backend,
		Store:         spiffeFederationStore,
		EventsWatcher: eventsSvc,
		Clock:         clock,
		Logger:        logger,
	})
}
