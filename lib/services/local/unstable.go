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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

const assertionTTL = time.Minute * 10

// UnstableService is a catch-all for unstable backend operations related to migrations/compatibility
// that don't fit into, or merit the change of, one of the primary service interfaces.
type UnstableService struct {
	backend.Backend
	*AssertionReplayService
}

// NewUnstableService returns new unstable service instance.
func NewUnstableService(backend backend.Backend, assertion *AssertionReplayService) UnstableService {
	return UnstableService{backend, assertion}
}

// AssertSystemRole is not a stable part of the public API. Used by agents to
// prove that they have a given system role when their credentials originate from multiple
// separate join tokens so that they can be issued an instance certificate that encompasses
// all of their capabilities. This method will be deprecated once we have a more comprehensive
// model for join token joining/replacement.
func (s UnstableService) AssertSystemRole(ctx context.Context, req proto.SystemRoleAssertion) error {
	key := systemRoleAssertionsKey(req.ServerID, req.AssertionID)
	item, err := s.Get(ctx, key)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	var set proto.SystemRoleAssertionSet
	if err == nil {
		if err := utils.FastUnmarshal(item.Value, &set); err != nil {
			return trace.Wrap(err)
		}
	}

	set.ServerID = req.ServerID
	set.AssertionID = req.AssertionID
	set.SystemRoles = append(set.SystemRoles, req.SystemRole)

	newValue, err := utils.FastMarshal(&set)
	if err != nil {
		return trace.Wrap(err)
	}
	newItem := backend.Item{
		Key:     key,
		Value:   newValue,
		Expires: time.Now().Add(assertionTTL).UTC(),
	}
	if item != nil {
		_, err = s.CompareAndSwap(ctx, *item, newItem)
		if trace.IsCompareFailed(err) {
			// nodes are expected to perform assertions sequentially
			return trace.CompareFailed("system role assertion set was concurrently modified (this is bug)")
		}
		return trace.Wrap(err)
	}

	_, err = s.Create(ctx, newItem)
	if trace.IsAlreadyExists(err) {
		// nodes are expected to perform assertions sequentially
		return trace.AlreadyExists("system role assertion set was concurrently created (this is a bug)")
	}
	return trace.Wrap(err)
}

// GetSystemRoleAssertionsis not a stable part of the auth API. Used in validated claims
// made by older instances to prove that they hold a given system role. This method will be
// deprecated once we have a more comprehensive model for join token joining/replacement.
func (s UnstableService) GetSystemRoleAssertions(ctx context.Context, serverID string, assertionID string) (proto.SystemRoleAssertionSet, error) {
	var set proto.SystemRoleAssertionSet

	item, err := s.Get(ctx, systemRoleAssertionsKey(serverID, assertionID))
	if err != nil {
		return set, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(item.Value, &set); err != nil {
		return set, trace.Wrap(err)
	}

	return set, nil
}

func systemRoleAssertionsKey(serverID string, assertionID string) backend.Key {
	return backend.NewKey(systemRoleAssertionsPrefix, serverID, assertionID)
}

const (
	systemRoleAssertionsPrefix = "system_role_assertions"
)
