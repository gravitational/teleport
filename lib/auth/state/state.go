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

package state

import (
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

const (
	// IdentityCurrent is a name for the identity credentials that are
	// currently used by the process.
	IdentityCurrent = "current"
	// IdentityReplacement is a name for the identity credentials that are
	// replacing current identity credentials during CA rotation.
	IdentityReplacement = "replacement"
	// stateName is an internal resource object name
	stateName = "state"
)

// StateV2 is a local process state.
type StateV2 struct {
	// ResourceHeader is a common resource header.
	types.ResourceHeader
	// Spec is a process spec.
	Spec StateSpecV2 `json:"spec"`
}

// GetInitialLocalVersion gets the initial local version string. If ok is false it indicates that
// this state value was written by a teleport agent that was too old to record the initial local version.
func (s *StateV2) GetInitialLocalVersion() (v string, ok bool) {
	return s.Spec.InitialLocalVersion, s.Spec.InitialLocalVersion != UnknownLocalVersion
}

// CheckAndSetDefaults checks and sets defaults values.
func (s *StateV2) CheckAndSetDefaults() error {
	s.Kind = types.KindState
	s.Version = types.V2
	// for state resource name does not matter
	if s.Metadata.Name == "" {
		s.Metadata.Name = stateName
	}
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if s.Spec.InitialLocalVersion == "" {
		return trace.BadParameter("agent identity state must specify initial local version")
	}

	if v, ok := s.GetInitialLocalVersion(); ok {
		if _, err := semver.NewVersion(v); err != nil {
			return trace.BadParameter("malformed initial local version %q: %v", s.Spec.InitialLocalVersion, err)
		}
	}

	return nil
}

// UnknownLocalVersion is a sentinel value used to distinguish between InitialLocalVersion being missing from
// state due to malformed input and InitialLocalVersion being missing due to the state having been created before
// teleport started recording InitialLocalVersion.
const UnknownLocalVersion = "unknown"

// StateSpecV2 is a state spec.
type StateSpecV2 struct {
	// Rotation holds local process rotation state.
	Rotation types.Rotation `json:"rotation"`

	// InitialLocalVersion records the version of teleport that initially
	// wrote this state to disk.
	InitialLocalVersion string `json:"initial_local_version,omitempty"`
}

// IdentityV2 specifies local host identity.
type IdentityV2 struct {
	// ResourceHeader is a common resource header.
	types.ResourceHeader
	// Spec is the identity spec.
	Spec IdentitySpecV2 `json:"spec"`
}

// CheckAndSetDefaults checks and sets defaults values.
func (s *IdentityV2) CheckAndSetDefaults() error {
	s.Kind = types.KindIdentity
	s.Version = types.V2
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if len(s.Spec.Key) == 0 {
		return trace.BadParameter("missing parameter Key")
	}
	if len(s.Spec.SSHCert) == 0 {
		return trace.BadParameter("missing parameter SSHCert")
	}
	if len(s.Spec.TLSCert) == 0 {
		return trace.BadParameter("missing parameter TLSCert")
	}
	if len(s.Spec.TLSCACerts) == 0 {
		return trace.BadParameter("missing parameter TLSCACerts")
	}
	if len(s.Spec.SSHCACerts) == 0 {
		return trace.BadParameter("missing parameter SSH CA bytes")
	}
	return nil
}

// IdentitySpecV2 specifies credentials used by local process.
type IdentitySpecV2 struct {
	// Key is a PEM encoded private key.
	Key []byte `json:"key,omitempty"`
	// SSHCert is a PEM encoded SSH host cert.
	SSHCert []byte `json:"ssh_cert,omitempty"`
	// TLSCert is a PEM encoded x509 client certificate.
	TLSCert []byte `json:"tls_cert,omitempty"`
	// TLSCACert is a list of PEM encoded x509 certificate of the
	// certificate authority of the cluster.
	TLSCACerts [][]byte `json:"tls_ca_certs,omitempty"`
	// SSHCACerts is a list of SSH certificate authorities encoded in the
	// authorized_keys format.
	SSHCACerts [][]byte `json:"ssh_ca_certs,omitempty"`
}
