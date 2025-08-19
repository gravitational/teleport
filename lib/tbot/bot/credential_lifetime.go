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

package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

// DefaultCredentialLifetime are the default TTL and RenewalInterval values for
// the bot's credentials.
var DefaultCredentialLifetime = CredentialLifetime{
	TTL:             60 * time.Minute,
	RenewalInterval: 20 * time.Minute,
}

// CredentialLifetime contains configuration for how long credentials will
// last (TTL) and the frequency at which they'll be renewed (RenewalInterval).
//
// It's a member on the BotConfig and service/output config structs, marked with
// the `inline` YAML tag so its fields become individual fields in the YAML
// config format.
type CredentialLifetime struct {
	TTL             time.Duration `yaml:"credential_ttl,omitempty"`
	RenewalInterval time.Duration `yaml:"renewal_interval,omitempty"`

	// SkipMaxTTLValidation is used by services that do not abide by standard
	// teleport credential lifetime limits to override the check that the
	// user specified TTL is less than the max TTL. For example, X509 SVIDs can
	// be issued with a lifetime of up to 2 weeks.
	SkipMaxTTLValidation bool `yaml:"-"`
}

// IsEmpty returns whether none of the fields is set (i.e. it is unconfigured).
func (l CredentialLifetime) IsEmpty() bool {
	// We don't care about this field being set when checking empty state.
	l.SkipMaxTTLValidation = false
	return l == CredentialLifetime{}
}

// Validate checks whether the combination of the fields is valid.
func (l CredentialLifetime) Validate(oneShot bool) error {
	if l.IsEmpty() {
		return nil
	}

	if l.TTL == 0 || l.RenewalInterval == 0 {
		return trace.BadParameter("credential_ttl and renewal_interval must both be specified if either is")
	}

	if l.TTL < 0 {
		return trace.BadParameter("credential_ttl must be positive")
	}

	if l.RenewalInterval < 0 {
		return trace.BadParameter("renewal_interval must be positive")
	}

	if l.TTL < l.RenewalInterval && !oneShot {
		return SuboptimalCredentialTTLError{
			msg: "Credential TTL is shorter than the renewal interval. This is likely an invalid configuration. Increase the credential TTL or decrease the renewal interval",
			details: map[string]any{
				"ttl":      l.TTL,
				"interval": l.RenewalInterval,
			},
		}
	}

	if !l.SkipMaxTTLValidation && l.TTL > defaults.MaxRenewableCertTTL {
		return SuboptimalCredentialTTLError{
			msg: "Requested certificate TTL exceeds the maximum TTL allowed and will likely be reduced by the Teleport server",
			details: map[string]any{
				"requested_ttl": l.TTL,
				"maximum_ttl":   defaults.MaxRenewableCertTTL,
			},
		}
	}

	return nil
}

// SuboptimalCredentialTTLError is returned from CredentialLifetime.Validate
// when the user has set CredentialTTL to something unusual that we can work
// around (e.g. if they exceed MaxRenewableCertTTL the server will reduce it)
// rather than rejecting their configuration.
//
// In the future, these probably *should* be hard failures - but that would be
// a breaking change.
type SuboptimalCredentialTTLError struct {
	msg     string
	details map[string]any
}

// Error satisfies the error interface.
func (e SuboptimalCredentialTTLError) Error() string {
	if len(e.details) == 0 {
		return e.msg
	}
	parts := make([]string, 0, len(e.details))
	for k, v := range e.details {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return fmt.Sprintf("%s (%s)", e.msg, strings.Join(parts, ", "))
}

// Message returns the error message without details.
func (e SuboptimalCredentialTTLError) Message() string {
	return e.msg
}

// LogLabels returns the error's details as a slice that can be passed as the
// varadic args parameter to log functions.
func (e SuboptimalCredentialTTLError) LogLabels() []any {
	labels := make([]any, 0, len(e.details)*2)
	for k, v := range e.details {
		labels = append(labels, k, v)
	}
	return labels
}
