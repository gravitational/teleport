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
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const WorkloadIdentityX509OutputType = "workload-identity-x509"

var (
	_ ServiceConfig = &WorkloadIdentityX509Service{}
	_ Initable      = &WorkloadIdentityX509Service{}
)

// WorkloadIdentityX509Service is the configuration for the WorkloadIdentityX509Service
// Emulates the output of https://github.com/spiffe/spiffe-helper
type WorkloadIdentityX509Service struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Selector is the selector for the WorkloadIdentity resource that will be
	// used to issue WICs.
	Selector bot.WorkloadIdentitySelector `yaml:"selector"`
	// Destination is where the credentials should be written to.
	Destination destination.Destination `yaml:"destination"`
	// IncludeFederatedTrustBundles controls whether to include federated trust
	// bundles in the output.
	IncludeFederatedTrustBundles bool `yaml:"include_federated_trust_bundles,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o WorkloadIdentityX509Service) GetName() string {
	return o.Name
}

// Init initializes the destination.
func (o *WorkloadIdentityX509Service) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// GetDestination returns the destination.
func (o *WorkloadIdentityX509Service) GetDestination() destination.Destination {
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
func (o *WorkloadIdentityX509Service) Describe() []bot.FileDescription {
	fds := []bot.FileDescription{
		{
			Name: SVIDPEMPath,
		},
		{
			Name: SVIDKeyPEMPath,
		},
		{
			Name: SVIDTrustBundlePEMPath,
		},
		{
			Name: SVIDCRLPemPath,
		},
	}
	return fds
}

func (o *WorkloadIdentityX509Service) Type() string {
	return WorkloadIdentityX509OutputType
}

// MarshalYAML marshals the WorkloadIdentityX509Service into YAML.
func (o *WorkloadIdentityX509Service) MarshalYAML() (any, error) {
	type raw WorkloadIdentityX509Service
	return encoding.WithTypeHeader((*raw)(o), WorkloadIdentityX509OutputType)
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

func (o *WorkloadIdentityX509Service) GetCredentialLifetime() bot.CredentialLifetime {
	lt := o.CredentialLifetime
	lt.SkipMaxTTLValidation = true
	return lt
}
