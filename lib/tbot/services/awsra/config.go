/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package awsra

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const (
	ServiceType                      = "workload-identity-aws-roles-anywhere"
	DefaultAWSSessionDuration        = 6 * time.Hour
	MaxAWSSessionDuration            = 12 * time.Hour
	DefaultAWSSessionRenewalInterval = 1 * time.Hour
)

// Config is the configuration for the Workload Identity AWS Roles Anywhere service.
type Config struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Selector is the selector for the WorkloadIdentity resource that will be
	// used to issue WICs.
	Selector bot.WorkloadIdentitySelector `yaml:"selector"`
	// Destination is where the credentials should be written to.
	Destination destination.Destination `yaml:"destination"`

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

// GetName returns the user-given name of the service, used for validation purposes.
func (o *Config) GetName() string {
	return o.Name
}

// SetName sets the service's name to an automatically generated one.
func (o *Config) SetName(name string) {
	o.Name = name
}

// Init initializes the destination.
func (o *Config) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// CheckAndSetDefaults checks the WorkloadIdentityAWSRAService values and sets any defaults.
func (o *Config) CheckAndSetDefaults() error {
	if o.Destination == nil {
		return trace.BadParameter("no destination configured for output")
	}
	if err := o.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating destination")
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
		o.SessionDuration = DefaultAWSSessionDuration
	}
	if o.SessionDuration > MaxAWSSessionDuration {
		return trace.BadParameter("session_duration: must be less than or equal to 12 hours")
	}
	if o.SessionRenewalInterval == 0 {
		o.SessionRenewalInterval = DefaultAWSSessionRenewalInterval
	}
	if o.SessionRenewalInterval >= o.SessionDuration {
		return trace.BadParameter("session_renewal_interval: must be less than session_duration")
	}

	return nil
}

// Describe returns the file descriptions for the service.
func (o *Config) Describe() []bot.FileDescription {
	// TODO: this is wrong/has been copy pasted from the JWT-SVID output.
	fds := []bot.FileDescription{
		{
			Name: internal.JWTSVIDPath,
		},
	}
	return fds
}

func (o *Config) Type() string {
	return ServiceType
}

// MarshalYAML marshals the WorkloadIdentityJWTService into YAML.
func (o *Config) MarshalYAML() (any, error) {
	type raw Config
	return encoding.WithTypeHeader((*raw)(o), ServiceType)
}

func (o *Config) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", o)
}

func (o *Config) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not implemented" error
	type raw Config
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

// GetDestination returns the destination.
func (o *Config) GetDestination() destination.Destination {
	return o.Destination
}

func (o *Config) GetCredentialLifetime() bot.CredentialLifetime {
	return bot.CredentialLifetime{}
}
