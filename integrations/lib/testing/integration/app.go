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

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunAndWaitReady is a helper to start an app implementing AppI and wait for
// it to become ready.
// This is used to start plugins.
func RunAndWaitReady(t *testing.T, app AppI) {
	appCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		ctx := appCtx
		if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			assert.ErrorContains(t, err, context.Canceled.Error(), "if a non-nil error is returned, it should be canceled context")
		}
	}()

	t.Cleanup(func() {
		err := app.Shutdown(appCtx)
		assert.NoError(t, err)
		err = app.Err()
		if err != nil {
			assert.ErrorContains(t, err, context.Canceled.Error(), "if a non-nil error is returned, it should be canceled context")
		}
	})

	waitCtx, cancel := context.WithTimeout(appCtx, 20*time.Second)
	defer cancel()

	ok, err := app.WaitReady(waitCtx)
	require.NoError(t, err)
	require.True(t, ok)
}
