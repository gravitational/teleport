/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// OpenIDConnectProviderCreator can create an OpenID Connect Identity Provider
// (OIDC IdP) in AWS IAM.
type OpenIDConnectProviderCreator interface {
	// CreateOpenIDConnectProvider creates an AWS IAM OIDC IdP.
	CreateOpenIDConnectProvider(ctx context.Context, params *iam.CreateOpenIDConnectProviderInput, optFns ...func(*iam.Options)) (*iam.CreateOpenIDConnectProviderOutput, error)
}

// CreateOIDCProvider wraps a [OpenIDConnectPRoviderCreator] in a
// [provisioning.Action] that creates an OIDC IdP in AWS IAM when invoked.
func CreateOIDCProvider(
	clt OpenIDConnectProviderCreator,
	thumbprints []string,
	issuerURL string,
	clientIDs []string,
	tags tags.AWSTags,
) (*provisioning.Action, error) {
	input := &iam.CreateOpenIDConnectProviderInput{
		ThumbprintList: thumbprints,
		Url:            &issuerURL,
		ClientIDList:   clientIDs,
		Tags:           tags.ToIAMTags(),
	}
	details, err := formatDetails(input)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name:    "CreateOpenIDConnectProvider",
		Summary: "Create an OpenID Connect identity provider in AWS IAM for your Teleport cluster",
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			slog.InfoContext(ctx, "Creating OpenID Connect identity provider")
			_, err = clt.CreateOpenIDConnectProvider(ctx, input)
			if err != nil {
				awsErr := awslib.ConvertIAMError(err)
				if trace.IsAlreadyExists(awsErr) {
					slog.InfoContext(ctx, "OpenID Connect identity provider already exists")
					return nil
				}

				return trace.Wrap(err)
			}
			return nil
		},
	}
	action, err := provisioning.NewAction(config)
	return action, trace.Wrap(err)
}
