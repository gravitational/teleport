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
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
)

const WorkloadIdentityAPIServiceType = "workload-identity-api"

var (
	_ ServiceConfig = &WorkloadIdentityAPIService{}
)

// WorkloadIdentityAPIService is the configuration for the
// WorkloadIdentityAPIService
type WorkloadIdentityAPIService struct {
	// Listen is the address on which the SPIFFE Workload API server should
	// listen. This should either be prefixed with "unix://" or "tcp://".
	Listen string `yaml:"listen"`
	// Attestors is the configuration for the workload attestation process.
	Attestors workloadattest.Config `yaml:"attestors"`
	// Selector is the selector for the WorkloadIdentity resource that
	// will be used to issue WICs.
	Selector WorkloadIdentitySelector `yaml:"selector"`

	// CredentialLifetime contains configuration for how long X.509 SVIDs will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime CredentialLifetime `yaml:",inline"`
}

// CheckAndSetDefaults checks the SPIFFESVIDOutput values and sets any defaults.
func (o *WorkloadIdentityAPIService) CheckAndSetDefaults() error {
	if o.Listen == "" {
		return trace.BadParameter("listen: should not be empty")
	}
	if err := o.Attestors.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating attestor")
	}
	if err := o.Selector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating selector")
	}
	return nil
}

// Type returns the type of the service.
func (o *WorkloadIdentityAPIService) Type() string {
	return WorkloadIdentityAPIServiceType
}

// MarshalYAML marshals the WorkloadIdentityOutput into YAML.
func (o *WorkloadIdentityAPIService) MarshalYAML() (interface{}, error) {
	type raw WorkloadIdentityAPIService
	return withTypeHeader((*raw)(o), WorkloadIdentityAPIServiceType)
}

// UnmarshalYAML unmarshals the WorkloadIdentityOutput from YAML.
func (o *WorkloadIdentityAPIService) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw WorkloadIdentityAPIService
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (o *WorkloadIdentityAPIService) GetCredentialLifetime() CredentialLifetime {
	return o.CredentialLifetime
}
