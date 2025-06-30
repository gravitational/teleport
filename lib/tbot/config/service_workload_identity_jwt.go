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

const WorkloadIdentityJWTOutputType = "workload-identity-jwt"

var (
	_ ServiceConfig = &WorkloadIdentityJWTService{}
	_ Initable      = &WorkloadIdentityJWTService{}
)

// WorkloadIdentityJWTService is the configuration for the WorkloadIdentityJWTService
type WorkloadIdentityJWTService struct {
	// Selector is the selector for the WorkloadIdentity resource that will be
	// used to issue WICs.
	Selector WorkloadIdentitySelector `yaml:"selector"`
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// Audiences is the list of audiences that the JWT should be valid for.
	Audiences []string

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime CredentialLifetime `yaml:",inline"`
}

// Init initializes the destination.
func (o *WorkloadIdentityJWTService) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// CheckAndSetDefaults checks the WorkloadIdentityJWTService values and sets any defaults.
func (o *WorkloadIdentityJWTService) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if err := o.Selector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating selector")
	}
	if len(o.Audiences) == 0 {
		return trace.BadParameter("audiences: must have at least one value")
	}
	return nil
}

// JWTSVIDPath is the name of the artifact that a JWT SVID will be written to.
const JWTSVIDPath = "jwt_svid"

// Describe returns the file descriptions for the WorkloadIdentityJWTService.
func (o *WorkloadIdentityJWTService) Describe() []FileDescription {
	fds := []FileDescription{
		{
			Name: JWTSVIDPath,
		},
	}
	return fds
}

func (o *WorkloadIdentityJWTService) Type() string {
	return WorkloadIdentityJWTOutputType
}

// MarshalYAML marshals the WorkloadIdentityJWTService into YAML.
func (o *WorkloadIdentityJWTService) MarshalYAML() (any, error) {
	type raw WorkloadIdentityJWTService
	return withTypeHeader((*raw)(o), WorkloadIdentityJWTOutputType)
}

// UnmarshalYAML unmarshals the WorkloadIdentityJWTService from YAML.
func (o *WorkloadIdentityJWTService) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw WorkloadIdentityJWTService
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

// GetDestination returns the destination.
func (o *WorkloadIdentityJWTService) GetDestination() bot.Destination {
	return o.Destination
}

func (o *WorkloadIdentityJWTService) GetCredentialLifetime() CredentialLifetime {
	lt := o.CredentialLifetime
	lt.skipMaxTTLValidation = true
	return lt
}
