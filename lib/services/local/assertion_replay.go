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

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
)

const assertionReplayPrefix = "recognized_assertions"

// AssertionReplayService tracks used SSO assertions to mitigate replay attacks.
// Assertions are automatically derecognized when their signed expiry passes.
type AssertionReplayService struct {
	bk backend.Backend
}

// Recognize a new assertion until it becomes invalid.
// This will error with `trace.AlreadyExists` if the assertion has been previously recognized.
func (s *AssertionReplayService) Recognize(ctx context.Context, assertionId string, user string, safeAfter time.Time) error {
	key := backend.Key(assertionReplayPrefix, assertionId)
	item := backend.Item{Key: key, Value: []byte(user), Expires: safeAfter.Add(time.Hour)}
	_, err := s.bk.Create(ctx, item)
	switch {
	case trace.IsAlreadyExists(err):
		return trace.AlreadyExists("Assertion %q already recognized for user %v", assertionId, user)
	case err != nil:
		return trace.Wrap(err)
	default:
		return nil
	}
}
