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

package aws_sync

import (
	"context"
	"encoding/xml"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/gravitational/trace"
	samltypes "github.com/russellhaering/gosaml2/types"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func (a *awsFetcher) pollAWSSAMLProviders(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error

		iamClient, err := a.CloudClients.GetAWSIAMClient(
			ctx,
			"", /* region is empty because saml providers are global */
			a.getAWSOptions()...,
		)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to get AWS IAM client"))
			return nil
		}
		listResp, err := iamClient.ListSAMLProvidersWithContext(ctx, &iam.ListSAMLProvidersInput{})
		if err != nil {
			collectErr(trace.Wrap(err, "failed to list AWS SAML identity providers"))
			return nil
		}

		providers := make([]*accessgraphv1alpha.AWSSAMLProviderV1, 0, len(listResp.SAMLProviderList))
		for _, providerRef := range listResp.SAMLProviderList {
			arn := aws.StringValue(providerRef.Arn)
			provider, err := a.fetchAWSSAMLProvider(ctx, iamClient, arn)
			if err != nil {
				collectErr(trace.Wrap(err, "failed to get info for SAML provider %s", arn))
			} else {
				providers = append(providers, provider)
			}
		}

		result.SAMLProviders = providers
		return nil
	}
}

// fetchAWSSAMLProvider fetches data about a single SAML identity provider.
func (a *awsFetcher) fetchAWSSAMLProvider(ctx context.Context, client iamiface.IAMAPI, arn string) (*accessgraphv1alpha.AWSSAMLProviderV1, error) {
	providerResp, err := client.GetSAMLProviderWithContext(ctx, &iam.GetSAMLProviderInput{
		SAMLProviderArn: aws.String(arn),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := awsSAMLProviderOutputToProto(arn, a.AccountID, providerResp)
	return out, trace.Wrap(err)
}

// awsSAMLProviderToSAML converts an iam.SAMLProvider to accessgraphv1alpha.SAMLProviderV1 representation.
func awsSAMLProviderOutputToProto(arn string, accountID string, provider *iam.GetSAMLProviderOutput) (*accessgraphv1alpha.AWSSAMLProviderV1, error) {
	var tags []*accessgraphv1alpha.AWSTag
	for _, v := range provider.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.StringValue(v.Key),
			Value: strPtrToWrapper(v.Value),
		})
	}

	var metadata samltypes.EntityDescriptor
	if err := xml.Unmarshal([]byte(aws.StringValue(provider.SAMLMetadataDocument)), &metadata); err != nil {
		return nil, trace.Wrap(err, "failed to parse SAML metadata for %s", arn)
	}

	var ssoURLs []string
	if metadata.IDPSSODescriptor == nil {
		return nil, trace.BadParameter("metadata for %v did not contain IdP descriptor", arn)
	}
	for _, ssoService := range metadata.IDPSSODescriptor.SingleSignOnServices {
		ssoURLs = append(ssoURLs, ssoService.Location)
	}

	return &accessgraphv1alpha.AWSSAMLProviderV1{
		Arn:        arn,
		CreatedAt:  awsTimeToProtoTime(provider.CreateDate),
		ValidUntil: awsTimeToProtoTime(provider.ValidUntil),
		Tags:       tags,
		AccountId:  accountID,
		EntityId:   metadata.EntityID,
		SsoUrls:    ssoURLs,
	}, nil
}
