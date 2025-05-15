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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/tbot/bot"
)

const (
	WorkloadIdentityAWSRAType        = "workload-identity-aws-roles-anywhere"
	defaultAWSSessionDuration        = 6 * time.Hour
	maxAWSSessionDuration            = 12 * time.Hour
	defaultAWSSessionRenewalInterval = 1 * time.Hour
)

var (
	_ ServiceConfig = &WorkloadIdentityAWSRAService{}
	_ Initable      = &WorkloadIdentityAWSRAService{}
)

// WorkloadIdentityAWSRAService is the configuration for the
// WorkloadIdentityAWSRAService
type WorkloadIdentityAWSRAService struct {
	// Selector is the selector for the WorkloadIdentity resource that will be
	// used to issue WICs.
	Selector WorkloadIdentitySelector `yaml:"selector"`
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`

	// RoleARN is the ARN of the role to assume.
	// Example: `arn:aws:iam::123456789012:role/example-role`
	// Required.
	RoleARN string `yaml:"role_arn"`
	// ProfileARN is the ARN of the Roles Anywhere profile to use.
	// Example: `arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000`
	// Required.
	ProfileARN string `yaml:"profile_arn"`
	// TrustAnchorARN is the ARN of the Roles Anywhere trust anchor to use.
	// Example: `arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000`
	// Required.
	TrustAnchorARN string `yaml:"trust_anchor_arn"`
	// Region is the AWS region to use.
	// Example: `us-east-1`
	// Must be set here or in the environment or AWS config using the
	// `AWS_REGION` environment variable. If set here, this will override the
	// environment or AWS config.
	Region string `yaml:"region"`

	// SessionDuration is the duration of the resulting AWS session and
	// credentials. This may be up to 12 hours. When unset, this defaults to
	// 6 hours.
	SessionDuration time.Duration `yaml:"session_duration"`
	// SessionRenewalInterval is the interval at which the session should be
	// renewed. This should be less than the session duration. When unset, this
	// defaults to 1 hour.
	SessionRenewalInterval time.Duration `yaml:"session_renewal_interval"`

	// CredentialProfileName is the name of the AWS credentials profile to
	// write to. If unspecified, the profile will be named "default".
	CredentialProfileName string `yaml:"credential_profile_name,omitempty"`

	// ArtifactName is the name of the artifact to write to. This is the
	// filename of the file that will be written to the destination. This is
	// by default "aws_credentials".
	ArtifactName string `yaml:"artifact_name,omitempty"`
	// OverwriteCredentialFile is a flag that indicates whether the output
	// should overwrite the existing credentials file rather than merging with
	// it.
	OverwriteCredentialFile bool `yaml:"overwrite_credential_file,omitempty"`

	// EndpointOverride is the endpoint to use for the AWS Roles Anywhere service.
	// This is designed to be leveraged by tests and unset in production
	// circumstances.
	EndpointOverride string `yaml:"-"`
}

// Init initializes the destination.
func (o *WorkloadIdentityAWSRAService) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// CheckAndSetDefaults checks the WorkloadIdentityAWSRAService values and sets any defaults.
func (o *WorkloadIdentityAWSRAService) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if err := o.Selector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating selector")
	}

	switch {
	case o.RoleARN == "":
		return trace.BadParameter("role_arn: must be set")
	case o.ProfileARN == "":
		return trace.BadParameter("profile_arn: must be set")
	case o.TrustAnchorARN == "":
		return trace.BadParameter("trust_anchor_arn: must be set")
	}
	if _, err := arn.Parse(o.RoleARN); err != nil {
		return trace.Wrap(err, "parsing role_arn")
	}
	if _, err := arn.Parse(o.ProfileARN); err != nil {
		return trace.Wrap(err, "parsing profile_arn")
	}
	if _, err := arn.Parse(o.TrustAnchorARN); err != nil {
		return trace.Wrap(err, "parsing trust_anchor_arn")
	}
	if o.Region != "" {
		if err := aws.IsValidRegion(o.Region); err != nil {
			return trace.Wrap(err, "validating region")
		}
	}

	if o.SessionDuration == 0 {
		o.SessionDuration = defaultAWSSessionDuration
	}
	if o.SessionDuration > maxAWSSessionDuration {
		return trace.BadParameter("session_duration: must be less than or equal to 12 hours")
	}
	if o.SessionRenewalInterval == 0 {
		o.SessionRenewalInterval = defaultAWSSessionRenewalInterval
	}
	if o.SessionRenewalInterval >= o.SessionDuration {
		return trace.BadParameter("session_renewal_interval: must be less than session_duration")
	}

	return nil
}

// Describe returns the file descriptions for the WorkloadIdentityJWTService.
func (o *WorkloadIdentityAWSRAService) Describe() []FileDescription {
	fds := []FileDescription{
		{
			Name: JWTSVIDPath,
		},
	}
	return fds
}

func (o *WorkloadIdentityAWSRAService) Type() string {
	return WorkloadIdentityAWSRAType
}

// MarshalYAML marshals the WorkloadIdentityJWTService into YAML.
func (o *WorkloadIdentityAWSRAService) MarshalYAML() (interface{}, error) {
	type raw WorkloadIdentityAWSRAService
	return withTypeHeader((*raw)(o), WorkloadIdentityAWSRAType)
}

// UnmarshalYAML unmarshals the WorkloadIdentityJWTService from YAML.
func (o *WorkloadIdentityAWSRAService) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw WorkloadIdentityAWSRAService
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

// GetDestination returns the destination.
func (o *WorkloadIdentityAWSRAService) GetDestination() bot.Destination {
	return o.Destination
}

func (o *WorkloadIdentityAWSRAService) GetCredentialLifetime() CredentialLifetime {
	return CredentialLifetime{}
}
