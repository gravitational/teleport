/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"cmp"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/iterutils"
	"github.com/gravitational/teleport/lib/itertools"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// SSHAccessChecker provides SSH-specific access checking, abstracting over scoped and unscoped identities.
// It is obtained from [ScopedAccessChecker.SSH] and should not be constructed directly. Methods on this type
// implement SSH-specific behavior, branching internally between the scoped and unscoped paths of the underlying
// [ScopedAccessChecker].
type SSHAccessChecker struct {
	checker *ScopedAccessChecker
}

// CheckAccessToSSHServer checks access to an SSH server for the given OS user.
func (c *SSHAccessChecker) CheckAccessToSSHServer(target types.Server, state AccessState, osUser string) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, state, NewLoginMatcher(osUser))
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, state, NewLoginMatcher(osUser))
}

// CanAccessSSHServer checks whether read access to the specified SSH server is possible without
// regard to a specific OS user or MFA state. Used for listing/filtering.
func (c *SSHAccessChecker) CanAccessSSHServer(target types.Server) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, AccessState{MFAVerified: true})
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, AccessState{MFAVerified: true})
}

// AdjustClientIdleTimeout determines the SSH client idle timeout to apply. The supplied argument must be
// the globally defined most-permissive value. For scoped identities, the value is read directly from the
// scoped role proto (ssh.client_idle_timeout takes precedence over defaults.client_idle_timeout). If the
// role specifies a more restrictive value it is returned; otherwise the global value is returned unchanged.
// An error is returned if the role contains a non-empty duration string that cannot be parsed.
func (c *SSHAccessChecker) AdjustClientIdleTimeout(timeout time.Duration) (time.Duration, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustClientIdleTimeout(timeout), nil
	}

	// aggregate stream of all non-empty client_idle_timeout strings
	idleStrs := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (string, bool) {
		// ssh.client_idle_timeout takes precedence over defaults.client_idle_timeout
		s := cmp.Or(role.GetSpec().GetSsh().GetClientIdleTimeout(), role.GetSpec().GetDefaults().GetClientIdleTimeout())
		return s, s != ""
	})

	// convert to fallible stream of parsed durations
	decoded := stream.TryMap(stream.FromSeq(idleStrs), func(s string) (time.Duration, error) {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, trace.Wrap(err, "failed to parse client_idle_timeout %q", s)
		}
		return d, nil
	})

	// add the global timeout to the stream. since we always select the lowest non-zero value, the global
	// needs no special handling.
	durations := stream.Chain(decoded, stream.Once(timeout))

	// select the lowest non-zero duration
	return minNonZeroStream(durations)
}

// AdjustDisconnectExpiredCert adjusts whether to disconnect on certificate expiry. For scoped identities, an explicit
// true in any role takes precedence over an explicit false in any role. The supplied default is only used if no role
// specifies a value (ssh.disconnect_expired_cert takes precedence over defaults.disconnect_expired_cert).
func (c *SSHAccessChecker) AdjustDisconnectExpiredCert(disconnect bool) bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustDisconnectExpiredCert(disconnect)
	}

	// aggregate all explicitly set disconnect_expired_cert values
	disconnects := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (bool, bool) {
		if ssh := role.GetSpec().GetSsh(); ssh.HasDisconnectExpiredCert() {
			return ssh.GetDisconnectExpiredCert(), true
		}
		return role.GetSpec().GetDefaults().GetDisconnectExpiredCert(), role.GetSpec().GetDefaults().HasDisconnectExpiredCert()
	})

	// an explicit true takes precedence over an explicit false.
	if d, ok := highestPrecedence(disconnects, true, false); ok {
		return d
	}

	// the cluster default is only applied if no role specifies a value.
	return disconnect
}

// LockingMode returns the SSH lock enforcement mode to apply. For scoped identities, strict in any role takes
// precedence over best_effort in any role. The supplied default is only used if no role specifies a mode
// (ssh.lock takes precedence over defaults.lock).
func (c *SSHAccessChecker) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.LockingMode(defaultMode)
	}

	// aggregate all explicitly set lock modes
	modes := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (constants.LockingMode, bool) {
		lock := role.GetSpec().GetSsh().GetLock()
		if lock == nil {
			lock = role.GetSpec().GetDefaults().GetLock()
		}
		return constants.LockingMode(lock.GetMode()), lock.GetMode() != ""
	})

	// strict takes precedence over best effort
	if mode, ok := highestPrecedence(modes, constants.LockingModeStrict, constants.LockingModeBestEffort); ok {
		return mode
	}

	// the cluster default is only applied if no role specifies a recognized mode.
	return defaultMode
}

// SessionRecordingMode returns the session recording mode for SSH sessions. For scoped identities, strict in
// any role takes precedence, otherwise best_effort is used. Each role's ssh.session_recording takes precedence
// over its defaults.session_recording.
func (c *SSHAccessChecker) SessionRecordingMode() constants.SessionRecordingMode {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.SessionRecordingMode(constants.SessionRecordingServiceSSH)
	}

	// aggregate all explicitly set session recording modes
	modes := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (constants.SessionRecordingMode, bool) {
		sr := role.GetSpec().GetSsh().GetSessionRecording()
		if sr == nil {
			sr = role.GetSpec().GetDefaults().GetSessionRecording()
		}
		return constants.SessionRecordingMode(sr.GetMode()), sr.GetMode() != ""
	})

	// strict takes precedence over best effort
	if mode, ok := highestPrecedence(modes, constants.SessionRecordingModeStrict, constants.SessionRecordingModeBestEffort); ok {
		return mode
	}
	return constants.SessionRecordingModeBestEffort
}

// PermitX11Forwarding returns true if X11 forwarding is permitted. For scoped identities, X11 forwarding is
// permitted iff at least one role explicitly permits it and no role explicitly forbids it.
func (c *SSHAccessChecker) PermitX11Forwarding() bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.PermitX11Forwarding()
	}

	// aggregate all explicitly set permit_x11_forwarding values
	permits := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (bool, bool) {
		ssh := role.GetSpec().GetSsh()
		return ssh.GetPermitX11Forwarding(), ssh.HasPermitX11Forwarding()
	})

	// an explicit false takes precedence over an explicit true
	if permitted, ok := highestPrecedence(permits, false, true); ok {
		return permitted
	}

	// X11 forwarding is not permitted if no role specifies a value.
	return false
}

// SSHPortForwardMode returns the SSH port forwarding mode. For scoped identities, local and remote port forwarding
// are evaluated independently. Each is permitted iff at least one role explicitly enables it and no role explicitly
// disables it.
func (c *SSHAccessChecker) SSHPortForwardMode() decisionpb.SSHPortForwardMode {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.SSHPortForwardMode()
	}

	// aggregate all explicitly set local/remote port forwarding values
	locals := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (bool, bool) {
		l := role.GetSpec().GetSsh().GetPortForwarding().GetLocal()
		return l.GetEnabled(), l.HasEnabled()
	})
	remotes := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (bool, bool) {
		r := role.GetSpec().GetSsh().GetPortForwarding().GetRemote()
		return r.GetEnabled(), r.HasEnabled()
	})

	// an explicit false takes precedence over an explicit true.
	local, ok := highestPrecedence(locals, false, true)
	if !ok {
		// defaults to false if no role specifies a value.
		local = false
	}

	// an explicit false takes precedence over an explicit true.
	remote, ok := highestPrecedence(remotes, false, true)
	if !ok {
		// defaults to false if no role specifies a value.
		remote = false
	}

	switch {
	case local && remote:
		return decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON
	case local:
		return decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_LOCAL
	case remote:
		return decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_REMOTE
	default:
		return decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_OFF
	}
}

// HostSudoers returns the sudoers rules for the host. For scoped identities, the sudoers entries of all roles
// are unioned (deduplicated, preserving first occurrence).
func (c *SSHAccessChecker) HostSudoers(srv types.Server) ([]string, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.HostSudoers(srv)
	}

	templateCtx := RoleTemplateContext{
		Username: c.checker.AccessInfo().Username,
		Traits:   c.checker.Traits(),
		Scoped:   true,
	}

	// TODO(fspmarshall/scopes): sudoers ordering is significant (when multiple entries match, the last one wins), so
	// the ordering of entries summed across roles may affect the resulting permissions. Entries are currently emitted
	// in role iteration order (winning role first), meaning that entries from lower precedence roles may override
	// those of higher precedence roles. The correct ordering behavior is subject to further discussion.
	var sudoers []string
	seen := make(map[string]struct{})
	for role := range c.checker.roles() {
		// traits are interpolated per-role prior to deduplication so that distinct templates which expand
		// to the same value are deduplicated correctly.
		for _, sudoer := range ApplyValueTraitsSlice(role.GetSpec().GetSsh().GetHostSudoers(), templateCtx, "host_sudoers") {
			if _, ok := seen[sudoer]; ok {
				continue
			}
			seen[sudoer] = struct{}{}
			sudoers = append(sudoers, sudoer)
		}
	}
	return sudoers, nil
}

// EnhancedRecordingSet returns the set of enhanced session recording events to capture. For scoped identities,
// the events enabled by all roles are unioned.
func (c *SSHAccessChecker) EnhancedRecordingSet() map[string]bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.EnhancedRecordingSet()
	}
	m := make(map[string]bool)
	for role := range c.checker.roles() {
		events := role.GetSpec().GetSsh().GetEnhancedRecording()
		if events.GetCommand() {
			m[constants.EnhancedRecordingCommand] = true
		}
		if events.GetDisk() {
			m[constants.EnhancedRecordingDisk] = true
		}
		if events.GetNetwork() {
			m[constants.EnhancedRecordingNetwork] = true
		}
	}
	return m
}

// HostUsers returns host user creation information for the server, or a decision with nil info if host user creation
// is disabled. For scoped identities, host user creation is summed across roles as follows:
//   - An explicit mode of "off" in any role disables host user creation.
//   - Otherwise, "keep" in any role takes precedence over "insecure-drop".
//   - Roles that do not specify a mode do not contribute to the decision. If no role specifies a mode, host user
//     creation is disabled.
//   - Groups are unioned across all roles that enable host user creation.
//   - Shell is the first non-empty shell (in role iteration order) among roles that enable host user creation.
func (c *SSHAccessChecker) HostUsers(srv types.Server) (*decisionpb.HostUsersInfo, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.HostUsers(srv)
	}

	var (
		mode      types.CreateHostUserMode
		shell     string
		groups    []string
		allowedBy []*decisionpb.Determinant
		deniedBy  []*decisionpb.Determinant
		unsetBy   []*decisionpb.Determinant
	)
	seenGroups := make(map[string]struct{})

	templateCtx := RoleTemplateContext{
		Username: c.checker.AccessInfo().Username,
		Traits:   c.checker.Traits(),
		Scoped:   true,
	}

	for role := range c.checker.roles() {
		createHostUser := role.GetSpec().GetSsh().GetHostUserCreation()

		var roleMode types.CreateHostUserMode
		if err := roleMode.UnmarshalText([]byte(createHostUser.GetMode())); err != nil {
			return nil, trace.Wrap(err)
		}

		determinant := decisionpb.Determinant_builder{
			Kind:  role.GetKind(),
			Name:  role.GetMetadata().GetName(),
			Scope: role.GetScope(),
		}.Build()

		switch roleMode {
		case types.CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED:
			// roles that do not specify a mode do not contribute to the decision.
			unsetBy = append(unsetBy, determinant)
			continue
		case types.CreateHostUserMode_HOST_USER_MODE_OFF:
			deniedBy = append(deniedBy, determinant)
			continue
		case types.CreateHostUserMode_HOST_USER_MODE_KEEP:
			mode = types.CreateHostUserMode_HOST_USER_MODE_KEEP
		case types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP:
			// keep takes precedence over insecure-drop.
			if mode != types.CreateHostUserMode_HOST_USER_MODE_KEEP {
				mode = types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP
			}
		default:
			return nil, trace.BadParameter("unsupported host user creation mode %q in scoped role %q", createHostUser.GetMode(), role.GetMetadata().GetName())
		}

		allowedBy = append(allowedBy, determinant)

		if shell == "" {
			shell = createHostUser.GetShell()
		}

		// traits are interpolated per-role prior to deduplication so that distinct templates which expand
		// to the same value are deduplicated correctly.
		for _, group := range ApplyValueTraitsSlice(createHostUser.GetGroups(), templateCtx, "host_user_creation.groups") {
			if _, ok := seenGroups[group]; ok {
				continue
			}
			seenGroups[group] = struct{}{}
			groups = append(groups, group)
		}
	}

	// an explicit "off" in any role disables host user creation.
	if len(deniedBy) > 0 {
		return nil, trace.AccessDenied("host user creation is disabled by scoped role(s)")
	}

	// no role enabled host user creation, meaning all roles left mode unset.
	if len(allowedBy) == 0 {
		return nil, trace.AccessDenied("no scoped role authorizes host user creation")
	}

	traits := c.checker.Traits()
	var uid, gid string
	if uidL := traits[constants.TraitHostUserUID]; len(uidL) >= 1 {
		uid = uidL[0]
	}
	if gidL := traits[constants.TraitHostUserGID]; len(gidL) >= 1 {
		gid = gidL[0]
	}

	return decisionpb.HostUsersInfo_builder{
		Groups: groups,
		Mode:   convertHostUserMode(mode),
		Uid:    uid,
		Gid:    gid,
		Shell:  shell,
	}.Build(), nil
}

// CheckAgentForward checks whether SSH agent forwarding is permitted for the given login. For scoped identities,
// agent forwarding is permitted iff at least one role explicitly permits it and no role explicitly forbids it.
func (c *SSHAccessChecker) CheckAgentForward(login string) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAgentForward(login)
	}

	// aggregate all explicitly set forward_agent values
	permits := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (bool, bool) {
		ssh := role.GetSpec().GetSsh()
		return ssh.GetForwardAgent(), ssh.HasForwardAgent()
	})

	// an explicit false takes precedence over an explicit true
	if permitted, ok := highestPrecedence(permits, false, true); ok && permitted {
		return nil
	}

	// agent forwarding is not permitted if no role specifies a value.
	return trace.AccessDenied("agent forwarding is not permitted by scoped role(s)")
}

// MaxConnections returns the maximum number of concurrent SSH connections permitted.
// A value of zero means unconstrained.
//
// NOTE: this method does not implement inclusive summing as we have not yet implemented a
// proper scoped control for max connections.
func (c *SSHAccessChecker) MaxConnections() int64 {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.MaxConnections()
	}
	return c.checker.scopedCompatChecker.MaxConnections()
}

// MaxSessions returns the maximum number of concurrent SSH sessions per connection permitted.
// A value of zero means unconstrained. For scoped identities, the most restrictive non-zero
// value across all roles is used.
func (c *SSHAccessChecker) MaxSessions() int64 {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.MaxSessions()
	}
	return minNonZero(iterutils.Map(func(role *scopedaccessv1.ScopedRole) int64 {
		return role.GetSpec().GetSsh().GetMaxSessions()
	}, c.checker.roles()))
}

// GetAllowedLoginsForServer returns the OS logins permitted for the given server in order to populate
// certain user-facing hint values.
//
// NOTE: This method does not perform summing as it is only used for generating user-facing hints in
// a context where aggregate checkers are never in play.
func (c *SSHAccessChecker) GetAllowedLoginsForServer(server types.Server) ([]string, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.GetAllowedLoginsForResource(server)
	}
	return c.checker.scopedCompatChecker.GetAllowedLoginsForResource(server)
}

// getScopedLogins returns the OS logins permitted by this scoped role. Returns nil for unscoped
// identities, which aggregate logins differently via [CertificateParameterContext.GetSSHLoginsForTTL].
// This method is intentionally unexported to prevent accidental use outside cert-param aggregation.
//
// NOTE: this method does not implement summing because it is used only in cert-param generation
// which never produces aggregate access checkers.
func (c *SSHAccessChecker) getScopedLogins() []string {
	if !c.checker.isScoped() {
		return nil
	}
	return filterInvalidUnixLogins(ApplyValueTraitsSlice(
		c.checker.role.GetSpec().GetSsh().GetLogins(),
		RoleTemplateContext{
			Username: c.checker.AccessInfo().Username,
			Traits:   c.checker.Traits(),
			Scoped:   true,
		},
		"logins",
	))
}

// CanCopyFiles returns true if remote file operations via SCP or SFTP are permitted. For scoped identities,
// file copy is permitted iff at least one role explicitly permits it and no role explicitly forbids it.
func (c *SSHAccessChecker) CanCopyFiles() bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CanCopyFiles()
	}

	// aggregate all explicitly set file_copy values
	permits := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (bool, bool) {
		ssh := role.GetSpec().GetSsh()
		return ssh.GetFileCopy(), ssh.HasFileCopy()
	})

	// an explicit false takes precedence over an explicit true
	if permitted, ok := highestPrecedence(permits, false, true); ok {
		return permitted
	}

	// file copy is not permitted if no role specifies a value.
	return false
}
