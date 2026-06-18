/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/moderation"
)

// fakeCommandEvaluator is a test implementation of moderation.CommandEvaluator.
type fakeCommandEvaluator struct {
	result moderation.CommandEvaluationResult
}

func (f fakeCommandEvaluator) EvaluateCommand(ctx context.Context, req moderation.CommandEvaluationRequest) (moderation.CommandEvaluationResult, error) {
	return f.result, nil
}

func TestServer_CommandEvaluator(t *testing.T) {
	t.Parallel()

	// A freshly constructed Server has no evaluator registered (OSS default):
	// callers must fail closed.
	s := &Server{}
	require.Nil(t, s.GetCommandEvaluator(), "expected no evaluator registered by default")

	// After registration the same evaluator is returned.
	fake := fakeCommandEvaluator{result: moderation.CommandEvaluationResult{Approved: true, Reasoning: "ok"}}
	s.SetCommandEvaluator(fake)
	require.Equal(t, fake, s.GetCommandEvaluator())

	// It can be unregistered again.
	s.SetCommandEvaluator(nil)
	require.Nil(t, s.GetCommandEvaluator())
}
