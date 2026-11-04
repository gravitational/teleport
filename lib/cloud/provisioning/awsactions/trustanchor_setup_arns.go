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

package awsactions

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// TrustAnchorSetUpARNs prints the IAM Roles Anywhere set up ARNs to output.
func TrustAnchorSetUpARNs(
	clt interface {
		RolesAnywhereTrustAnchorLister
		RolesAnywhereProfileLister
		RoleGetter
	},
	trustAnchorName string,
	profileName string,
	roleName string,
	integrationURISAN string,
	output io.Writer,
) (*provisioning.Action, error) {

	type trustAnchorARNs struct {
		TrustAnchorName   string
		ProfileName       string
		RoleName          string
		IntegrationURISAN string
	}
	details, err := formatDetails(trustAnchorARNs{
		TrustAnchorName:   trustAnchorName,
		ProfileName:       profileName,
		RoleName:          roleName,
		IntegrationURISAN: integrationURISAN,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name:    "TrustAnchorSetUpARNs",
		Summary: "Prints the ARNs for the Trust Anchor, Profile, and Role",
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			trustAnchorDetails, err := trustAnchorDetails(ctx, trustAnchorName, clt)
			if err != nil {
				return trace.Wrap(err)
			}

			profileDetails, err := profileDetails(ctx, profileName, clt)
			if err != nil {
				return trace.Wrap(err)
			}

			roleDetails, err := clt.GetRole(ctx, &iam.GetRoleInput{
				RoleName: aws.String(roleName),
			})
			if err != nil {
				return trace.Wrap(err)
			}

			summaryBuilder := strings.Builder{}
			summaryBuilder.WriteString("\nCopy and paste the following values to Teleport UI\n\n")
			summaryBuilder.WriteString("=================================================\n")
			summaryBuilder.WriteString(aws.ToString(trustAnchorDetails.TrustAnchorArn) + "\n")
			summaryBuilder.WriteString(aws.ToString(profileDetails.ProfileArn) + "\n")
			summaryBuilder.WriteString(aws.ToString(roleDetails.Role.Arn) + "\n")
			summaryBuilder.WriteString("=================================================\n\n")
			if integrationURISAN != "" {
				summaryBuilder.WriteString("Require this URI SAN in the trust policy of every role the integration grants access to:\n\n")
				fmt.Fprintf(&summaryBuilder, "  %q: %q\n\n", awslib.ConditionKeyRolesAnywhereURISAN, integrationURISAN)
				fmt.Fprintf(&summaryBuilder, "To stop other scoped integrations from using the sync role, also add it to the trust policy of the %q role.\n\n", roleName)
			}

			_, err = io.WriteString(output, summaryBuilder.String())
			if err != nil {
				return trace.Wrap(err)
			}

			return nil
		},
	}
	action, err := provisioning.NewAction(config)
	return action, trace.Wrap(err)
}
