/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package events

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

type failingSealer struct {
	closed atomic.Bool
}

func (s *failingSealer) Seal(_ context.Context, _ []byte) ([]byte, bool, error) {
	return nil, false, trace.Errorf("sealer has no keys")
}

func (s *failingSealer) Close() error {
	s.closed.Store(true)
	return nil
}

func TestAsyncEmitterThreadsSealerIntoQueue(t *testing.T) {
	sealer := &failingSealer{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{
		Inner:            &unaryEmitter{},
		DataDir:          t.TempDir(),
		EnableAuditQueue: true,
		Sealer:           sealer,
	})
	require.NoError(t, err)

	event := &apievents.UserLogin{}
	event.SetID("a")
	err = a.EmitAuditEvent(t.Context(), event)
	require.ErrorContains(t, err, "sealer has no keys")

	require.NoError(t, a.Close())
	require.True(t, sealer.closed.Load(), "emitter must close the sealer it owns")
}
