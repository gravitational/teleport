/*
Copyright 2019 Gravitational, Inc.

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

package server

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// ProcessStorage is a backend for local process state,
// it helps to manage rotation for certificate authorities
// and keeps local process credentials - x509 and SSH certs and keys.
type ProcessStorage struct {
	backend.Backend
}

// Close closes all resources used by process storage backend.
func (p *ProcessStorage) Close() error {
	return p.Backend.Close()
}

const (
	// IdentityNameCurrent is a name for the identity credentials that are
	// currently used by the process.
	IdentityCurrent = "current"
	// IdentityReplacement is a name for the identity crdentials that are
	// replacing current identity credentials during CA rotation.
	IdentityReplacement = "replacement"
	// stateName is an internal resource object name
	stateName = "state"
	// statesPrefix is a key prefix for object states
	statesPrefix = "states"
	// idsPrefix is a key prefix for identities
	idsPrefix = "ids"
)

// GetState reads rotation state from disk.
func (p *ProcessStorage) GetState(role teleport.Role) (*StateV2, error) {
	item, err := p.Get(context.TODO(), backend.Key(statesPrefix, strings.ToLower(role.String()), stateName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var res StateV2
	if err := utils.UnmarshalWithSchema(resource.GetStateSchema(), &res, item.Value); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return &res, nil
}

// CreateState creates process state if it does not exist yet.
func (p *ProcessStorage) CreateState(role teleport.Role, state StateV2) error {
	if err := state.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(state)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.Key(statesPrefix, strings.ToLower(role.String()), stateName),
		Value: value,
	}
	_, err = p.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// WriteState writes local cluster state to the backend.
func (p *ProcessStorage) WriteState(role teleport.Role, state StateV2) error {
	if err := state.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(state)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.Key(statesPrefix, strings.ToLower(role.String()), stateName),
		Value: value,
	}
	_, err = p.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ReadIdentity reads identity using identity name and role.
func (p *ProcessStorage) ReadIdentity(name string, role teleport.Role) (*Identity, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := p.Get(context.TODO(), backend.Key(idsPrefix, strings.ToLower(role.String()), name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var res IdentityV2
	if err := utils.UnmarshalWithSchema(resource.GetIdentitySchema(), &res, item.Value); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return ReadIdentityFromKeyPair(&PackedKeys{
		Key:        res.Spec.Key,
		Cert:       res.Spec.SSHCert,
		TLSCert:    res.Spec.TLSCert,
		TLSCACerts: res.Spec.TLSCACerts,
		SSHCACerts: res.Spec.SSHCACerts,
	})
}

// WriteIdentity writes identity to the backend.
func (p *ProcessStorage) WriteIdentity(name string, id Identity) error {
	res := IdentityV2{
		ResourceHeader: services.ResourceHeader{
			Kind:    services.KindIdentity,
			Version: services.V2,
			Metadata: services.Metadata{
				Name: name,
			},
		},
		Spec: IdentitySpecV2{
			Key:        id.KeyBytes,
			SSHCert:    id.CertBytes,
			TLSCert:    id.TLSCertBytes,
			TLSCACerts: id.TLSCACertsBytes,
			SSHCACerts: id.SSHCACertBytes,
		},
	}
	if err := res.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(res)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.Key(idsPrefix, strings.ToLower(id.ID.Role.String()), name),
		Value: value,
	}
	_, err = p.Put(context.TODO(), item)
	return trace.Wrap(err)
}

// StateV2 is a local process state.
type StateV2 struct {
	// ResourceHeader is a common resource header.
	services.ResourceHeader
	// Spec is a process spec.
	Spec StateSpecV2 `json:"spec"`
}

// CheckAndSetDefaults checks and sets defaults values.
func (s *StateV2) CheckAndSetDefaults() error {
	s.Kind = services.KindState
	s.Version = services.V2
	// for state resource name does not matter
	if s.Metadata.Name == "" {
		s.Metadata.Name = stateName
	}
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StateSpecV2 is a state spec.
type StateSpecV2 struct {
	// Rotation holds local process rotation state.
	Rotation services.Rotation `json:"rotation"`
}

// IdentityV2 specifies local host identity.
type IdentityV2 struct {
	// ResourceHeader is a common resource header.
	services.ResourceHeader
	// Spec is the identity spec.
	Spec IdentitySpecV2 `json:"spec"`
}

// CheckAndSetDefaults checks and sets defaults values.
func (s *IdentityV2) CheckAndSetDefaults() error {
	s.Kind = services.KindIdentity
	s.Version = services.V2
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
