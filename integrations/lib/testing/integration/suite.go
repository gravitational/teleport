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

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport/integrations/lib/logger"
)

// Suite is a basic testing suite enhanced with context management.
type Suite struct {
	suite.Suite
	contexts map[*testing.T]contexts
}

// AppI is an app that can be spawned along with running test.
type AppI interface {
	// Run starts the application
	Run(ctx context.Context) error
	// WaitReady waits till the application finishes initialization
	WaitReady(ctx context.Context) (bool, error)
	// Err returns last error
	Err() error
	// Shutdown shuts the application down
	Shutdown(ctx context.Context) error
}

type contexts struct {
	// baseCtx is the base context for appCtx and testCtx.
	// It could store some test-specific information stored using context.WithValue()
	// such as test name for the logger etc.
	baseCtx context.Context

	// appCtx inherits from baseCtx. Its purpose is to limit the lifetime of the apps running in parallel.
	// By "app" we mean some plugin (e.g. access/slack) or the Teleport process (lib/testing/integration package).
	// Its timeout is slightly higher than testCtx's for a reason. When the test example fails with timeout
	// we want to see the exact line of the test file where the fail took place. But if the app dies at the same time
	// as the some operation in the test example we probably we'll see the line where the app failed, not the test
	// which is non-informative but we really want to see what line of the test caused the timeout and where it happened.
	appCtx context.Context

	// testCtx inherits from baseCtx. Its purpose is to limit the lifetime of the test method.
	// This context is guaranteed to be canceled earlier than appCtx for better error reporting (see explanation above).
	testCtx context.Context
}

// SetT sets the current *testing.T context.
func (s *Suite) SetT(t *testing.T) {
	oldT := s.T()
	s.Suite.SetT(t)
	s.initContexts(oldT, t)
}

func (s *Suite) initContexts(oldT *testing.T, newT *testing.T) {
	if s.contexts == nil {
		s.contexts = make(map[*testing.T]contexts)
	}
	contexts, ok := s.contexts[newT]
	if ok {
		// Context already initialized.
		// This happens when testify sets the parent context back after running a subtest.
		return
	}
	var baseCtx context.Context
	if oldT != nil && strings.HasPrefix(newT.Name(), oldT.Name()+"/") {
		// We are running a subtest so lets inherit the context too.
		baseCtx = s.contexts[oldT].testCtx
	} else {
		baseCtx = context.Background()
	}
	baseCtx, _ = logger.With(baseCtx, "test", newT.Name())
	baseCtx, cancel := context.WithCancel(baseCtx)
	newT.Cleanup(cancel)

	contexts.baseCtx = baseCtx
	contexts.appCtx = baseCtx
	contexts.testCtx = baseCtx

	// Just memoize the context in a map and that's all.
	// Lets not bother with cleaning up this storage, it's not gonna be that big.
	s.contexts[newT] = contexts
}

// SetContextTimeout limits the lifetime of test and app contexts.
func (s *Suite) SetContextTimeout(timeout time.Duration) context.Context {
	s.T().Helper()
	t := s.T()

	contexts, ok := s.contexts[t]
	require.True(t, ok)

	var cancel context.CancelFunc
	// We set appCtx timeout slightly higher than testCtx for test assertions to fall earlier than
	// app (plugin) fails.
	contexts.appCtx, cancel = context.WithTimeout(contexts.baseCtx, timeout+500*time.Millisecond)
	t.Cleanup(cancel)
	contexts.testCtx, cancel = context.WithTimeout(contexts.baseCtx, timeout)
	t.Cleanup(cancel)

	s.contexts[t] = contexts

	return contexts.testCtx
}

// Context returns a current test context.
func (s *Suite) Context() context.Context {
	s.T().Helper()
	t := s.T()
	contexts, ok := s.contexts[t]
	require.True(t, ok)
	return contexts.testCtx
}

// NewTmpFile creates a new temporary file.
func (s *Suite) NewTmpFile(pattern string) *os.File {
	s.T().Helper()
	t := s.T()

	file, err := os.CreateTemp("", pattern)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Remove(file.Name())
		require.NoError(t, err)
	})
	return file
}

// StartApp spawns an app in parallel with the running test/suite.
func (s *Suite) StartApp(app AppI) {
	s.T().Helper()
	t := s.T()

	contexts, ok := s.contexts[t]
	require.True(t, ok)

	go func() {
		ctx := contexts.appCtx
		if err := app.Run(ctx); err != nil {
			// We're in a goroutine so we can't just require.NoError(t, err).
			// All we can do is to log an error.
			logger.Get(ctx).ErrorContext(ctx, "Application failed", "error", err)
		}
	}()

	t.Cleanup(func() {
		err := app.Shutdown(contexts.appCtx)
		assert.NoError(t, err)
		assert.NoError(t, app.Err())
	})

	ok, err := app.WaitReady(contexts.testCtx)
	require.NoError(t, err)
	require.True(t, ok)
}
