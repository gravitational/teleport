// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

const WorkloadIdentityOutputType = "workload-identity"

var (
	_ ServiceConfig = &WorkloadIdentityOutput{}
	_ Initable      = &WorkloadIdentityOutput{}
)

// WorkloadIdentitySelector allows the user to select which WorkloadIdentity
// resource should be used.
type WorkloadIdentitySelector struct {
	Name string `yaml:"name"`
	// TODO(noah): Eventually, you'll also be able to alternatively specify
	// labels here.
}

func (s *WorkloadIdentitySelector) CheckAndSetDefaults() error {
	if s.Name == "" {
		return trace.BadParameter("name: should not be empty")
	}
	return nil
}

// WorkloadIdentityOutput is the configuration for the WorkloadIdentityOutput
// Emulates the output of https://github.com/spiffe/spiffe-helper
type WorkloadIdentityOutput struct {
	// WorkloadIdentity is the selector for the WorkloadIdentity resource that
	// will be used to issue WICs.
	WorkloadIdentity WorkloadIdentitySelector `yaml:"workload_identity"`
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// IncludeFederatedTrustBundles controls whether to include federated trust
	// bundles in the output.
	IncludeFederatedTrustBundles bool `yaml:"include_federated_trust_bundles,omitempty"`
	// JWTs is an optional list of audiences and file names to write JWT SVIDs
	// to. If none are specified, no JWT will be output.
	JWTAudiences []string `yaml:"jwt_audiences,omitempty"`
}

// Init initializes the destination.
func (o *WorkloadIdentityOutput) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// GetDestination returns the destination.
func (o *WorkloadIdentityOutput) GetDestination() bot.Destination {
	return o.Destination
}

// CheckAndSetDefaults checks the SPIFFESVIDOutput values and sets any defaults.
func (o *WorkloadIdentityOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if err := o.WorkloadIdentity.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating workload_identity")
	}
	return nil
}

// Describe returns the file descriptions for the SPIFFE SVID output.
func (o *WorkloadIdentityOutput) Describe() []FileDescription {
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
	if len(o.JWTAudiences) > 0 {
		fds = append(fds, FileDescription{Name: "jwt"})
	}
	return nil
}

func (o *WorkloadIdentityOutput) Type() string {
	return WorkloadIdentityOutputType
}

// MarshalYAML marshals the WorkloadIdentityOutput into YAML.
func (o *WorkloadIdentityOutput) MarshalYAML() (interface{}, error) {
	type raw WorkloadIdentityOutput
	return withTypeHeader((*raw)(o), WorkloadIdentityOutputType)
}

// UnmarshalYAML unmarshals the WorkloadIdentityOutput from YAML.
func (o *WorkloadIdentityOutput) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw WorkloadIdentityOutput
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}
