/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/modules"
)

// AWSSessionProvider defines a function that creates an AWS Session.
// It must use ambient credentials if Integration is empty.
// It must use Integration credentials otherwise.
type AWSSessionProvider func(ctx context.Context, region string, integration string) (*session.Session, error)

// StaticAWSSessionProvider is a helper method that returns a static session.
// Must not be used to provide sessions when using Integrations.
func StaticAWSSessionProvider(awsSession *session.Session) AWSSessionProvider {
	return func(ctx context.Context, region, integration string) (*session.Session, error) {
		if integration != "" {
			return nil, trace.BadParameter("integration %q is not allowed to use static sessions", integration)
		}
		return awsSession, nil
	}
}

// SessionProviderUsingAmbientCredentials returns an AWS Session using ambient credentials.
// This is in contrast with AWS Sessions that can be generated using an AWS OIDC Integration.
func SessionProviderUsingAmbientCredentials() AWSSessionProvider {
	return func(ctx context.Context, region, integration string) (*session.Session, error) {
		if integration != "" {
			return nil, trace.BadParameter("integration %q is not allowed to use ambient sessions", integration)
		}
		useFIPSEndpoint := endpoints.FIPSEndpointStateUnset
		if modules.GetModules().IsBoringBinary() {
			useFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
		}
		session, err := session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
			Config: aws.Config{
				UseFIPSEndpoint: useFIPSEndpoint,
			},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return session, nil
	}
}
