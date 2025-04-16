// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessManager_PrematureReturn(t *testing.T) {
	pm, pmCtx := newProcessManager()
	defer pm.Close()

	pm.AddCriticalBackgroundTask("premature return", func() error {
		return nil
	})
	pm.AddCriticalBackgroundTask("context-aware task", func() error {
		<-pmCtx.Done()
		return pmCtx.Err()
	})

	err := pm.Wait()
	require.ErrorContains(t, err, "critical task \"premature return\" exited prematurely")
	// Verify that the cancellation cause is propagated through the context.
	require.ErrorIs(t, err, context.Cause(pmCtx))
}

func TestProcessManager_ReturnWithError(t *testing.T) {
	pm, pmCtx := newProcessManager()
	defer pm.Close()

	expectedErr := fmt.Errorf("lorem ipsum dolor sit amet")
	pm.AddCriticalBackgroundTask("return with error", func() error {
		return expectedErr
	})
	pm.AddCriticalBackgroundTask("context-aware task", func() error {
		<-pmCtx.Done()
		return pmCtx.Err()
	})

	err := pm.Wait()
	require.ErrorIs(t, err, expectedErr)
	require.ErrorIs(t, err, context.Cause(pmCtx))
}

func TestProcessManager_Close(t *testing.T) {
	pm, pmCtx := newProcessManager()
	defer pm.Close()

	pm.AddCriticalBackgroundTask("context-aware task", func() error {
		<-pmCtx.Done()
		return pmCtx.Err()
	})

	pm.Close()
	err := pm.Wait()
	require.NoError(t, err)
}
