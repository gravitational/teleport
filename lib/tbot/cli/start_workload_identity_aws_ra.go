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

package cli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/services/awsra"
)

// WorkloadIdentityAWSRACommand implements `tbot start workload-identity-aws-ra`
// and `tbot configure workload-identity-aws-ra`.
type WorkloadIdentityAWSRACommand struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	// NameSelector is the name of the workload identity to use.
	// --workload-identity-name foo
	NameSelector string
	// LabelSelector is the labels of the workload identity to use.
	// --workload-identity-labels x=y,z=a
	LabelSelector string

	// RoleARN is the ARN of the role to assume.
	// Example: `arn:aws:iam::123456789012:role/example-role`
	// Required.
	RoleARN string
	// ProfileARN is the ARN of the Roles Anywhere profile to use.
	// Example: `arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000`
	// Required.
	ProfileARN string
	// TrustAnchorARN is the ARN of the Roles Anywhere trust anchor to use.
	// Example: `arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000`
	// Required.
	TrustAnchorARN string
	// Region is the AWS region to use.
	// Example: `us-east-1`
	// Must be set here or in the environment or AWS config using the
	// `AWS_REGION` environment variable. If set here, this will override the
	// environment or AWS config.
	Region string

	// SessionDuration is the duration of the resulting AWS session and
	// credentials. This may be up to 12 hours. When unset, this defaults to
	// 6 hours.
	SessionDuration time.Duration
	// SessionRenewalInterval is the interval at which the session should be
	// renewed. This should be less than the session duration. When unset, this
	// defaults to 1 hour.
	SessionRenewalInterval time.Duration
}

// NewWorkloadIdentityAWSRACommand initializes the command and flags for the
// `workload-identity-aws-ra` output and returns a struct that will contain the parse
// result.
func NewWorkloadIdentityAWSRACommand(
	parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode,
) *WorkloadIdentityAWSRACommand {
	cmd := parentCmd.Command(
		"workload-identity-aws-roles-anywhere",
		fmt.Sprintf(
			"%s tbot with an output containing AWS credentials generated via AWS Roles Anywhere.",
			mode,
		),
	)

	c := &WorkloadIdentityAWSRACommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag(
		"name-selector",
		"The name of the workload identity to issue",
	).StringVar(&c.NameSelector)
	cmd.Flag(
		"label-selector",
		"A label-based selector for which workload identities to issue. Multiple labels can be provided using ','.",
	).StringVar(&c.LabelSelector)
	cmd.Flag(
		"role-arn",
		"The ARN of the role to assume.",
	).Required().StringVar(&c.RoleARN)
	cmd.Flag(
		"profile-arn",
		"The ARN of the Roles Anywhere profile to use.",
	).Required().StringVar(&c.ProfileARN)
	cmd.Flag(
		"trust-anchor-arn",
		"The ARN of the Roles Anywhere trust anchor to use.",
	).Required().StringVar(&c.TrustAnchorARN)
	cmd.Flag(
		"region",
		"The AWS region to use. If unset, value will be used from the AWS config or the AWS_REGION environment variable.",
	).StringVar(&c.Region)

	cmd.Flag(
		"session-duration",
		"The duration of the resulting AWS session and credentials. This may be up to 12 hours. When unset, this defaults to 6 hours.",
	).DurationVar(&c.SessionDuration)
	cmd.Flag(
		"session-renewal-interval",
		"How often the session should be renewed. This should be less than the session duration. When unset, this defaults to 1 hour.",
	).DurationVar(&c.SessionRenewalInterval)

	return c
}

// ApplyConfig applies the parsed flags to the bot configuration.
func (c *WorkloadIdentityAWSRACommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	svc := &awsra.Config{
		Destination:            dest,
		RoleARN:                c.RoleARN,
		ProfileARN:             c.ProfileARN,
		TrustAnchorARN:         c.TrustAnchorARN,
		Region:                 c.Region,
		SessionDuration:        c.SessionDuration,
		SessionRenewalInterval: c.SessionRenewalInterval,
	}

	switch {
	case c.NameSelector != "" && c.LabelSelector != "":
		return trace.BadParameter("name-selector and label-selector flags are mutually exclusive")
	case c.NameSelector != "":
		svc.Selector.Name = c.NameSelector
	case c.LabelSelector != "":
		labels, err := client.ParseLabelSpec(c.LabelSelector)
		if err != nil {
			return trace.Wrap(err, "parsing --label-selector")
		}
		svc.Selector.Labels = map[string][]string{}
		for k, v := range labels {
			svc.Selector.Labels[k] = []string{v}
		}
	default:
		return trace.BadParameter("name-selector or label-selector must be specified")
	}

	cfg.Services = append(cfg.Services, svc)

	return nil
}
