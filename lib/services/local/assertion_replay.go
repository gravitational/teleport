/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
