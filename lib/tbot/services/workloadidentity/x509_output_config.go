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

package workloadidentity

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const X509OutputServiceType = "workload-identity-x509"

// X509OutputConfig is the configuration for the Workload Identity x509 output
// service. // Emulates the output of https://github.com/spiffe/spiffe-helper
type X509OutputConfig struct {
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
func (o X509OutputConfig) GetName() string {
	return o.Name
}

// SetName sets the service's name to an automatically generated one.
func (o *X509OutputConfig) SetName(name string) {
	o.Name = name
}

// Init initializes the destination.
func (o *X509OutputConfig) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// GetDestination returns the destination.
func (o *X509OutputConfig) GetDestination() destination.Destination {
	return o.Destination
}

// CheckAndSetDefaults checks the SPIFFESVIDOutput values and sets any defaults.
func (o *X509OutputConfig) CheckAndSetDefaults() error {
	if o.Destination == nil {
		return trace.BadParameter("no destination configured for output")
	}
	if err := o.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating destination")
	}
	if err := o.Selector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating selector")
	}
	return nil
}

// Describe returns the file descriptions for the WorkloadIdentityX509Service.
func (o *X509OutputConfig) Describe() []bot.FileDescription {
	fds := []bot.FileDescription{
		{
			Name: internal.SVIDPEMPath,
		},
		{
			Name: internal.SVIDKeyPEMPath,
		},
		{
			Name: internal.SVIDTrustBundlePEMPath,
		},
		{
			Name: internal.SVIDCRLPemPath,
		},
	}
	return fds
}

func (o *X509OutputConfig) Type() string {
	return X509OutputServiceType
}

// MarshalYAML marshals the WorkloadIdentityX509Service into YAML.
func (o *X509OutputConfig) MarshalYAML() (any, error) {
	type raw X509OutputConfig
	return encoding.WithTypeHeader((*raw)(o), X509OutputServiceType)
}

func (o *X509OutputConfig) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", o)
}

func (o *X509OutputConfig) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not implemented" error
	type raw X509OutputConfig
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *X509OutputConfig) GetCredentialLifetime() bot.CredentialLifetime {
	lt := o.CredentialLifetime
	lt.SkipMaxTTLValidation = true
	return lt
}
