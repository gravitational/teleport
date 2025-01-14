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
	_ ServiceConfig = &WorkloadIdentityX509Service{}
	_ Initable      = &WorkloadIdentityX509Service{}
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

// WorkloadIdentityX509Service is the configuration for the WorkloadIdentityX509Service
// Emulates the output of https://github.com/spiffe/spiffe-helper
type WorkloadIdentityX509Service struct {
	// Selector is the selector for the WorkloadIdentity resource that will be
	// used to issue WICs.
	Selector WorkloadIdentitySelector `yaml:"selector"`
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// IncludeFederatedTrustBundles controls whether to include federated trust
	// bundles in the output.
	IncludeFederatedTrustBundles bool `yaml:"include_federated_trust_bundles,omitempty"`
}

// Init initializes the destination.
func (o *WorkloadIdentityX509Service) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// GetDestination returns the destination.
func (o *WorkloadIdentityX509Service) GetDestination() bot.Destination {
	return o.Destination
}

// CheckAndSetDefaults checks the SPIFFESVIDOutput values and sets any defaults.
func (o *WorkloadIdentityX509Service) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if err := o.Selector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating selector")
	}
	return nil
}

// Describe returns the file descriptions for the WorkloadIdentityX509Service.
func (o *WorkloadIdentityX509Service) Describe() []FileDescription {
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

func (o *WorkloadIdentityX509Service) Type() string {
	return WorkloadIdentityX509OutputType
}

// MarshalYAML marshals the WorkloadIdentityX509Service into YAML.
func (o *WorkloadIdentityX509Service) MarshalYAML() (interface{}, error) {
	type raw WorkloadIdentityX509Service
	return withTypeHeader((*raw)(o), WorkloadIdentityX509OutputType)
}

// UnmarshalYAML unmarshals the WorkloadIdentityX509Service from YAML.
func (o *WorkloadIdentityX509Service) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw WorkloadIdentityX509Service
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}
