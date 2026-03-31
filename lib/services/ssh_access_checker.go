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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
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
	// SSH block takes precedence over defaults block.
	idleStr := c.checker.role.GetSpec().GetSsh().GetClientIdleTimeout()
	if idleStr == "" {
		idleStr = c.checker.role.GetSpec().GetDefaults().GetClientIdleTimeout()
	}
	if idleStr != "" {
		d, err := time.ParseDuration(idleStr)
		if err != nil {
			return 0, trace.Errorf("invalid client_idle_timeout %q in scoped role %q: %w", idleStr, c.checker.role.GetMetadata().GetName(), err)
		}
		if d > 0 && (timeout == 0 || d < timeout) {
			return max(d, 0), nil
		}
	}
	return max(timeout, 0), nil
}

// AdjustDisconnectExpiredCert adjusts whether to disconnect on certificate expiry.
func (c *SSHAccessChecker) AdjustDisconnectExpiredCert(disconnect bool) bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustDisconnectExpiredCert(disconnect)
	}
	return c.checker.scopedCompatChecker.AdjustDisconnectExpiredCert(disconnect)
}

// SessionRecordingMode returns the session recording mode for SSH sessions.
func (c *SSHAccessChecker) SessionRecordingMode() constants.SessionRecordingMode {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.SessionRecordingMode(constants.SessionRecordingServiceSSH)
	}
	return c.checker.scopedCompatChecker.SessionRecordingMode(constants.SessionRecordingServiceSSH)
}

// CanPortForward returns true if port forwarding is permitted.
func (c *SSHAccessChecker) CanPortForward() bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CanPortForward()
	}
	return c.checker.scopedCompatChecker.CanPortForward()
}

// CanForwardAgents returns true if SSH agent forwarding is permitted.
func (c *SSHAccessChecker) CanForwardAgents() bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CanForwardAgents()
	}
	return c.checker.scopedCompatChecker.CanForwardAgents()
}

// PermitX11Forwarding returns true if X11 forwarding is permitted.
func (c *SSHAccessChecker) PermitX11Forwarding() bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.PermitX11Forwarding()
	}
	return c.checker.role.GetSpec().GetSsh().GetPermitX11Forwarding()
}

// SSHPortForwardMode returns the SSH port forwarding mode.
func (c *SSHAccessChecker) SSHPortForwardMode() decisionpb.SSHPortForwardMode {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.SSHPortForwardMode()
	}
	return c.checker.scopedCompatChecker.SSHPortForwardMode()
}

// HostSudoers returns the sudoers rules for the host.
func (c *SSHAccessChecker) HostSudoers(srv types.Server) ([]string, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.HostSudoers(srv)
	}
	return c.checker.scopedCompatChecker.HostSudoers(srv)
}

// EnhancedRecordingSet returns the set of enhanced session recording events to capture.
func (c *SSHAccessChecker) EnhancedRecordingSet() map[string]bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.EnhancedRecordingSet()
	}
	return c.checker.scopedCompatChecker.EnhancedRecordingSet()
}

// HostUsers returns host user creation information for the server, or nil if host user creation is disabled.
func (c *SSHAccessChecker) HostUsers(srv types.Server) (*HostUsersDecision, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.HostUsers(srv)
	}
	return c.checker.scopedCompatChecker.HostUsers(srv)
}

// CheckAgentForward checks whether SSH agent forwarding is permitted for the given login.
func (c *SSHAccessChecker) CheckAgentForward(login string) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAgentForward(login)
	}
	return c.checker.scopedCompatChecker.CheckAgentForward(login)
}

// MaxConnections returns the maximum number of concurrent SSH connections permitted.
// A value of zero means unconstrained.
func (c *SSHAccessChecker) MaxConnections() int64 {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.MaxConnections()
	}
	return c.checker.scopedCompatChecker.MaxConnections()
}

// MaxSessions returns the maximum number of concurrent SSH sessions per connection permitted.
// A value of zero means unconstrained.
func (c *SSHAccessChecker) MaxSessions() int64 {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.MaxSessions()
	}
	return c.checker.scopedCompatChecker.MaxSessions()
}

// getScopedLogins returns the OS logins permitted by this scoped role. Returns nil for unscoped
// identities, which aggregate logins differently via [CertificateParameterContext.GetSSHLoginsForTTL].
// This method is intentionally unexported to prevent accidental use outside cert-param aggregation.
func (c *SSHAccessChecker) getScopedLogins() []string {
	if !c.checker.isScoped() {
		return nil
	}
	return c.checker.role.GetSpec().GetSsh().GetLogins()
}

// CanCopyFiles returns true if remote file operations via SCP or SFTP are permitted.
func (c *SSHAccessChecker) CanCopyFiles() bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CanCopyFiles()
	}
	return c.checker.scopedCompatChecker.CanCopyFiles()
}
