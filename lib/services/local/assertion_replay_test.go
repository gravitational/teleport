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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestAssertionReplayService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	delay := func(t time.Duration) time.Time { return time.Now().UTC().Add(t) }
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service := NewAssertionReplayService(bk)
	id := make([]string, 2)
	for i := range id {
		id[i] = uuid.New().String()
	}

	// first time foo
	require.NoError(t, service.RecognizeSSOAssertion(ctx, "", id[0], "foo", delay(time.Hour)))

	// second time foo
	require.Error(t, service.RecognizeSSOAssertion(ctx, "", id[0], "foo", delay(time.Hour)))

	// first time bar
	require.NoError(t, service.RecognizeSSOAssertion(ctx, "", id[1], "bar", delay(time.Millisecond)))
	time.Sleep(time.Second)

	// assertion has expired, no risk of replay
	require.NoError(t, service.RecognizeSSOAssertion(ctx, "", id[1], "bar", delay(time.Hour)))

	// assertion should still exist
	require.Error(t, service.RecognizeSSOAssertion(ctx, "", id[1], "bar", delay(time.Hour)))
}
