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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend"
)

const assertionReplayPrefix = "recognized_assertions"

// AssertionReplayService tracks used SSO assertions to mitigate replay attacks.
// Assertions are automatically derecognized when their signed expiry passes.
type AssertionReplayService struct {
	bk backend.Backend
}

// NewAssertionReplayService creates a new instance of AssertionReplayService.
func NewAssertionReplayService(bk backend.Backend) *AssertionReplayService {
	return &AssertionReplayService{bk: bk}
}

// RecognizeSSOAssertion will remember a new assertion until it becomes invalid.
// This will error with `trace.AlreadyExists` if the assertion has been previously recognized.
//
// `safeAfter` must be either at or after the point in time that a given SSO assertion becomes invalid in order to mitigate replay attacks.
// This function shouldn't be used if the assertion never verifiably expires.
func (s *AssertionReplayService) RecognizeSSOAssertion(ctx context.Context, connectorID string, assertionID string, user string, safeAfter time.Time) error {
	key := backend.NewKey(assertionReplayPrefix, connectorID, assertionID)
	item := backend.Item{Key: key, Value: []byte(user), Expires: safeAfter}
	_, err := s.bk.Create(ctx, item)
	switch {
	case trace.IsAlreadyExists(err):
		return trace.AlreadyExists("Assertion %q already recognized for user %v", assertionID, user)
	case err != nil:
		return trace.Wrap(err)
	default:
		return nil
	}
}
