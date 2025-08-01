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

package services

import (
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// LockInForceAccessDenied is an AccessDenied error returned when a lock
// is in force.
func LockInForceAccessDenied(lock types.Lock) error {
	s := fmt.Sprintf("lock targeting %v is in force", lock.Target())
	msg := lock.Message()
	if len(msg) > 0 {
		s += ": " + msg
	}
	err := trace.AccessDenied("%s", s)
	return trace.WithField(err, "lock-in-force", lock)
}

// StrictLockingModeAccessDenied is an AccessDenied error returned when strict
// locking mode causes all interactions to be blocked.
var StrictLockingModeAccessDenied = trace.AccessDenied("preventive lock-out due to local lock view becoming unreliable")

// SSHAccessLockTargets computes the full set of lock targets related to ssh access.
func SSHAccessLockTargets(localClusterName, serverID, osLogin string, accessInfo *AccessInfo, unmappedIdentity *sshca.Identity) []types.LockTarget {
	// ssh access lock targets are currently constructed identically to proxying lock targets,
	// except that the os login associated with the ssh access attempt is also locked.
	return append(ProxyingLockTargets(localClusterName, serverID, accessInfo, unmappedIdentity), types.LockTarget{Login: osLogin})
}

// ProxyingLockTargets computes the full set of lock targets related to teleport proxying.
func ProxyingLockTargets(localClusterName, serverID string, accessInfo *AccessInfo, unmappedIdentity *sshca.Identity) []types.LockTarget {
	lockTargets := []types.LockTarget{
		{User: accessInfo.Username},
		{ServerID: serverID},
		{ServerID: utils.HostFQDN(serverID, localClusterName)},
	}
	if mfaDevice := unmappedIdentity.MFAVerified; mfaDevice != "" {
		lockTargets = append(lockTargets, types.LockTarget{MFADevice: mfaDevice})
	}
	if trustedDevice := unmappedIdentity.DeviceID; trustedDevice != "" {
		lockTargets = append(lockTargets, types.LockTarget{Device: trustedDevice})
	}
	if joinToken := unmappedIdentity.JoinToken; joinToken != "" {
		lockTargets = append(lockTargets, types.LockTarget{JoinToken: joinToken})
	}
	if botInstanceID := unmappedIdentity.BotInstanceID; botInstanceID != "" {
		lockTargets = append(lockTargets, types.LockTarget{BotInstanceID: botInstanceID})
	}
	roles := apiutils.Deduplicate(append(accessInfo.Roles, unmappedIdentity.Roles...))
	lockTargets = append(lockTargets, RolesToLockTargets(roles)...)
	lockTargets = append(lockTargets, AccessRequestsToLockTargets(unmappedIdentity.ActiveRequests)...)
	return lockTargets
}

// GitForwardingLockTargets computes the full set of lock targets related to git forwarding.
func GitForwardingLockTargets(localClusterName, serverID string, accessInfo *AccessInfo, unmappedIdentity *sshca.Identity) []types.LockTarget {
	// git forwarding lock targets are currently constructed identically to proxying lock targets.
	return ProxyingLockTargets(localClusterName, serverID, accessInfo, unmappedIdentity)
}

// LockTargetsFromTLSIdentity infers a list of LockTargets from tlsca.Identity.
func LockTargetsFromTLSIdentity(id tlsca.Identity) []types.LockTarget {
	lockTargets := append(RolesToLockTargets(id.Groups), types.LockTarget{User: id.Username})
	if id.MFAVerified != "" {
		lockTargets = append(lockTargets, types.LockTarget{MFADevice: id.MFAVerified})
	}
	if id.DeviceExtensions.DeviceID != "" {
		lockTargets = append(lockTargets, types.LockTarget{Device: id.DeviceExtensions.DeviceID})
	}
	if id.JoinToken != "" {
		lockTargets = append(lockTargets, types.LockTarget{JoinToken: id.JoinToken})
	}
	if id.BotInstanceID != "" {
		lockTargets = append(lockTargets, types.LockTarget{BotInstanceID: id.BotInstanceID})
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
		return nil, trace.BadParameter("%s", err)
	}
	if err := lock.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		lock.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		lock.SetExpiry(cfg.Expires)
	}
	return &lock, nil
}

// MarshalLock marshals the Lock resource to JSON.
func MarshalLock(lock types.Lock, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch lock := lock.(type) {
	case *types.LockV2:
		if err := lock.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if version := lock.GetVersion(); version != types.V2 {
			return nil, trace.BadParameter("mismatched lock version %v and type %T", version, lock)
		}
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, lock))
	default:
		return nil, trace.BadParameter("unrecognized lock version %T", lock)
	}
}
