/*
Copyright 2021 Gravitational, Inc.

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

package services

import (
	"fmt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// LockInForceAccessDenied is an AccessDenied error returned when a lock
// is in force.
func LockInForceAccessDenied(lock types.Lock) error {
	s := fmt.Sprintf("lock targeting %v is in force", lock.Target())
	msg := lock.Message()
	if len(msg) > 0 {
		s += ": " + msg
	}
	err := trace.AccessDenied(s)
	err.AddField("lock-in-force", lock)
	return err
}

// StrictLockingModeAccessDenied is an AccessDenied error returned when strict
// locking mode causes all interactions to be blocked.
var StrictLockingModeAccessDenied = trace.AccessDenied("preventive lock-out due to local lock view becoming unreliable")

// LockTargetsFromTLSIdentity infers a list of LockTargets from tlsca.Identity.
func LockTargetsFromTLSIdentity(id tlsca.Identity) []types.LockTarget {
	lockTargets := append(RolesToLockTargets(id.Groups), types.LockTarget{User: id.Username})
	if id.MFAVerified != "" {
		lockTargets = append(lockTargets, types.LockTarget{MFADevice: id.MFAVerified})
	}
	lockTargets = append(lockTargets, AccessRequestsToLockTargets(id.ActiveRequests)...)
	return lockTargets
}

// RolesToLockTargets converts a list of roles to a list of LockTargets
// (one LockTarget per role).
func RolesToLockTargets(roles []string) []types.LockTarget {
	lockTargets := make([]types.LockTarget, 0, len(roles))
	for _, role := range roles {
		lockTargets = append(lockTargets, types.LockTarget{Role: role})
	}
	return lockTargets
}

// AccessRequestsToLockTargets converts a list of access requests to a list of
// LockTargets (one LockTarget per access request)
func AccessRequestsToLockTargets(accessRequests []string) []types.LockTarget {
	lockTargets := make([]types.LockTarget, 0, len(accessRequests))
	for _, accessRequest := range accessRequests {
		lockTargets = append(lockTargets, types.LockTarget{AccessRequest: accessRequest})
	}
	return lockTargets
}

// UnmarshalLock unmarshals the Lock resource from JSON.
func UnmarshalLock(bytes []byte, opts ...MarshalOption) (types.Lock, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var lock types.LockV2
	if err := utils.FastUnmarshal(bytes, &lock); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := lock.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		lock.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		lock.SetExpiry(cfg.Expires)
	}
	return &lock, nil
}

// MarshalLock marshals the Lock resource to JSON.
func MarshalLock(lock types.Lock, opts ...MarshalOption) ([]byte, error) {
	if err := lock.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch lock := lock.(type) {
	case *types.LockV2:
		if version := lock.GetVersion(); version != types.V2 {
			return nil, trace.BadParameter("mismatched lock version %v and type %T", version, lock)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *lock
			copy.SetResourceID(0)
			lock = &copy
		}
		return utils.FastMarshal(lock)
	default:
		return nil, trace.BadParameter("unrecognized lock version %T", lock)
	}
}
