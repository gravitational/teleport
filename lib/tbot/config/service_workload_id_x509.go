// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package config

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

const WorkloadIdentityX509OutputType = "workload-identity-x509"

var (
	_ ServiceConfig = &WorkloadIdentityX509Output{}
	_ Initable      = &WorkloadIdentityX509Output{}
)

// WorkloadIdentitySelector allows the user to select which WorkloadIdentity
// resource should be used.
//
// Only one of Name or Labels can be set.
type WorkloadIdentitySelector struct {
	// Name is the name of a specific WorkloadIdentity resource.
	Name string `yaml:"name"`
	// Labels is a set of labels that the WorkloadIdentity resource must have.
	Labels map[string][]string `yaml:"labels,omitempty"`
}

// CheckAndSetDefaults checks the WorkloadIdentitySelector values and sets any
// defaults.
func (s *WorkloadIdentitySelector) CheckAndSetDefaults() error {
	switch {
	case s.Name == "" && len(s.Labels) == 0:
		return trace.BadParameter("one of ['name', 'labels'] must be set")
	case s.Name != "" && len(s.Labels) > 0:
		return trace.BadParameter("at most one of ['name', 'labels'] can be set")
	}
	for k, v := range s.Labels {
		if len(v) == 0 {
			return trace.BadParameter("labels[%s]: must have at least one value", k)
		}
	}
	return nil
}

// WorkloadIdentityX509Output is the configuration for the WorkloadIdentityX509Output
// Emulates the output of https://github.com/spiffe/spiffe-helper
type WorkloadIdentityX509Output struct {
	// WorkloadIdentity is the selector for the WorkloadIdentity resource that
	// will be used to issue WICs.
	WorkloadIdentity WorkloadIdentitySelector `yaml:"workload_identity"`
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// IncludeFederatedTrustBundles controls whether to include federated trust
	// bundles in the output.
	IncludeFederatedTrustBundles bool `yaml:"include_federated_trust_bundles,omitempty"`
}

// Init initializes the destination.
func (o *WorkloadIdentityX509Output) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// GetDestination returns the destination.
func (o *WorkloadIdentityX509Output) GetDestination() bot.Destination {
	return o.Destination
}

// CheckAndSetDefaults checks the SPIFFESVIDOutput values and sets any defaults.
func (o *WorkloadIdentityX509Output) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if err := o.WorkloadIdentity.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating workload_identity")
	}
	return nil
}

// Describe returns the file descriptions for the WorkloadIdentityX509Output.
func (o *WorkloadIdentityX509Output) Describe() []FileDescription {
	fds := []FileDescription{
		{
			Name: SVIDPEMPath,
		},
		{
			Name: SVIDKeyPEMPath,
		},
		{
			Name: SVIDTrustBundlePEMPath,
		},
	}
	return fds
}

func (o *WorkloadIdentityX509Output) Type() string {
	return WorkloadIdentityX509OutputType
}

// MarshalYAML marshals the WorkloadIdentityX509Output into YAML.
func (o *WorkloadIdentityX509Output) MarshalYAML() (interface{}, error) {
	type raw WorkloadIdentityX509Output
	return withTypeHeader((*raw)(o), WorkloadIdentityX509OutputType)
}

// UnmarshalYAML unmarshals the WorkloadIdentityX509Output from YAML.
func (o *WorkloadIdentityX509Output) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw WorkloadIdentityX509Output
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}
