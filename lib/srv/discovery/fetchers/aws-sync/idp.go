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
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func (a *Fetcher) pollAWSSAMLProviders(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		existing := a.lastResult
		iamClient, err := a.CloudClients.GetAWSIAMClient(
			ctx,
			"", /* region is empty because saml providers are global */
			a.getAWSOptions()...,
		)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to get AWS IAM client"))
			result.SAMLProviders = existing.SAMLProviders
			return nil
		}
		listResp, err := iamClient.ListSAMLProvidersWithContext(ctx, &iam.ListSAMLProvidersInput{})
		if err != nil {
			collectErr(trace.Wrap(err, "failed to list AWS SAML identity providers"))
			result.SAMLProviders = existing.SAMLProviders
			return nil
		}

		providers := make([]*accessgraphv1alpha.AWSSAMLProviderV1, 0, len(listResp.SAMLProviderList))
		for _, providerRef := range listResp.SAMLProviderList {
			arn := aws.StringValue(providerRef.Arn)
			provider, err := a.fetchAWSSAMLProvider(ctx, iamClient, arn)
			if err != nil {
				collectErr(trace.Wrap(err, "failed to get info for SAML provider %s", arn))
				provider = sliceFilterPickFirst(existing.SAMLProviders, func(p *accessgraphv1alpha.AWSSAMLProviderV1) bool {
					return p.Arn == arn
				})
			}
			if provider != nil {
				providers = append(providers, provider)
			}
		}

		result.SAMLProviders = providers
		return nil
	}
}

// fetchAWSSAMLProvider fetches data about a single SAML identity provider.
func (a *Fetcher) fetchAWSSAMLProvider(ctx context.Context, client iamiface.IAMAPI, arn string) (*accessgraphv1alpha.AWSSAMLProviderV1, error) {
	providerResp, err := client.GetSAMLProviderWithContext(ctx, &iam.GetSAMLProviderInput{
		SAMLProviderArn: aws.String(arn),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := awsSAMLProviderOutputToProto(arn, a.AccountID, providerResp)
	return out, trace.Wrap(err)
}

// awsSAMLProviderToProto converts an iam.SAMLProvider to accessgraphv1alpha.SAMLProviderV1 representation.
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

	const signingUse = "signing"
	var signingCerts []string
	for _, key := range metadata.IDPSSODescriptor.KeyDescriptors {
		ki := key.KeyInfo
		if key.Use == signingUse {
			for _, cert := range ki.X509Data.X509Certificates {
				signingCerts = append(signingCerts, cert.Data)
			}
		}
	}

	return &accessgraphv1alpha.AWSSAMLProviderV1{
		Arn:                 arn,
		CreatedAt:           awsTimeToProtoTime(provider.CreateDate),
		ValidUntil:          awsTimeToProtoTime(provider.ValidUntil),
		Tags:                tags,
		AccountId:           accountID,
		EntityId:            metadata.EntityID,
		SsoUrls:             ssoURLs,
		SigningCertificates: signingCerts,
		LastSyncTime:        timestamppb.Now(),
	}, nil
}

func (a *Fetcher) pollAWSOIDCProviders(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		existing := a.lastResult
		iamClient, err := a.CloudClients.GetAWSIAMClient(
			ctx,
			"", /* region is empty because oidc providers are global */
			a.getAWSOptions()...,
		)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to get AWS IAM client"))
			result.OIDCProviders = existing.OIDCProviders
			return nil
		}
		listResp, err := iamClient.ListOpenIDConnectProvidersWithContext(ctx, &iam.ListOpenIDConnectProvidersInput{})
		if err != nil {
			collectErr(trace.Wrap(err, "failed to list AWS OIDC identity providers"))
			result.OIDCProviders = existing.OIDCProviders
			return nil
		}

		providers := make([]*accessgraphv1alpha.AWSOIDCProviderV1, 0, len(listResp.OpenIDConnectProviderList))
		for _, providerRef := range listResp.OpenIDConnectProviderList {
			arn := aws.StringValue(providerRef.Arn)
			provider, err := a.fetchAWSOIDCProvider(ctx, iamClient, arn)
			if err != nil {
				collectErr(trace.Wrap(err, "failed to get info for OIDC provider %s", arn))
				provider = sliceFilterPickFirst(existing.OIDCProviders, func(p *accessgraphv1alpha.AWSOIDCProviderV1) bool {
					return p.Arn == arn
				})
			}
			if provider != nil {
				providers = append(providers, provider)
			}
		}

		result.OIDCProviders = providers
		return nil
	}
}

// fetchAWSOIDCProvider fetches data about a single OIDC identity provider.
func (a *Fetcher) fetchAWSOIDCProvider(ctx context.Context, client iamiface.IAMAPI, arn string) (*accessgraphv1alpha.AWSOIDCProviderV1, error) {
	providerResp, err := client.GetOpenIDConnectProviderWithContext(ctx, &iam.GetOpenIDConnectProviderInput{
		OpenIDConnectProviderArn: aws.String(arn),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := awsOIDCProviderOutputToProto(arn, a.AccountID, providerResp)
	return out, trace.Wrap(err)
}

// awsOIDCProviderToOIDC converts an iam.OpenIDConnectProvider to accessgraphv1alpha.OIDCProviderV1 representation.
func awsOIDCProviderOutputToProto(arn string, accountID string, provider *iam.GetOpenIDConnectProviderOutput) (*accessgraphv1alpha.AWSOIDCProviderV1, error) {
	var tags []*accessgraphv1alpha.AWSTag
	for _, v := range provider.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.StringValue(v.Key),
			Value: strPtrToWrapper(v.Value),
		})
	}

	return &accessgraphv1alpha.AWSOIDCProviderV1{
		Arn:          arn,
		CreatedAt:    awsTimeToProtoTime(provider.CreateDate),
		Tags:         tags,
		AccountId:    accountID,
		ClientIds:    aws.StringValueSlice(provider.ClientIDList),
		Thumbprints:  aws.StringValueSlice(provider.ThumbprintList),
		Url:          aws.StringValue(provider.Url),
		LastSyncTime: timestamppb.Now(),
	}, nil
}
