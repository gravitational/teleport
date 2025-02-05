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
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

func TestAWSIAMPollSAMLProviders(t *testing.T) {
	const accountID = "12345678"
	var regions = []string{"eu-west-1"}

	timestamp1 := time.Date(2024, time.May, 1, 1, 2, 3, 0, time.UTC)
	timestamp2 := timestamp1.AddDate(1, 0, 0)

	mockedClients := &cloud.TestCloudClients{
		IAM: &mocks.IAMMock{
			SAMLProviders: samlProviders(timestamp1, timestamp2),
		},
	}

	var (
		errs []error
		mu   sync.Mutex
	)

	collectErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}
	a := &Fetcher{
		Config: Config{
			AccountID:    accountID,
			CloudClients: mockedClients,
			Regions:      regions,
			Integration:  accountID,
		},
		lastResult: &Resources{},
	}
	expected := []*accessgraphv1alpha.AWSSAMLProviderV1{
		{
			Arn:        "arn:aws:iam::1234678:saml-provider/provider1",
			CreatedAt:  timestamppb.New(timestamp1),
			ValidUntil: timestamppb.New(timestamp2),
			Tags: []*accessgraphv1alpha.AWSTag{
				{Key: "key1", Value: &wrapperspb.StringValue{Value: "value1"}},
				{Key: "key2", Value: &wrapperspb.StringValue{Value: "value2"}},
			},
			AccountId:           accountID,
			EntityId:            "provider1",
			SsoUrls:             []string{"https://posturl.example.com", "https://redirecturl.example.com"},
			SigningCertificates: []string{"cert1", "cert2"},
		},
		{
			Arn:       "arn:aws:iam::1234678:saml-provider/provider2",
			CreatedAt: timestamppb.New(timestamp2),
			AccountId: accountID,
			EntityId:  "provider2",
			SsoUrls:   []string{"https://posturl.teleport.local", "https://redirecturl.teleport.local"},
		},
	}
	result := &Resources{}
	execFunc := a.pollAWSSAMLProviders(context.Background(), result, collectErr)
	require.NoError(t, execFunc())
	require.Empty(t, errs)
	sortByARN(result.SAMLProviders)
	require.Empty(t, cmp.Diff(
		expected,
		result.SAMLProviders,
		protocmp.Transform(),
		protocmp.IgnoreFields(&accessgraphv1alpha.AWSSAMLProviderV1{}, "last_sync_time"),
	))
}

func samlProviders(timestamp1, timestamp2 time.Time) map[string]*iam.GetSAMLProviderOutput {
	return map[string]*iam.GetSAMLProviderOutput{
		"arn:aws:iam::1234678:saml-provider/provider1": {
			CreateDate: aws.Time(timestamp1),
			SAMLMetadataDocument: aws.String(`<?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="provider1">
      <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
        <md:KeyDescriptor use="signing">
          <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
            <ds:X509Data>
              <ds:X509Certificate>cert1</ds:X509Certificate>
            </ds:X509Data>
          </ds:KeyInfo>
        </md:KeyDescriptor>
        <md:KeyDescriptor use="signing">
          <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
            <ds:X509Data>
              <ds:X509Certificate>cert2</ds:X509Certificate>
            </ds:X509Data>
          </ds:KeyInfo>
        </md:KeyDescriptor>
        <md:KeyDescriptor use="encryption">
          <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
            <ds:X509Data>
              <ds:X509Certificate>irrelevant_cert</ds:X509Certificate>
            </ds:X509Data>
          </ds:KeyInfo>
        </md:KeyDescriptor>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
		<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://posturl.example.com" />
		<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://redirecturl.example.com" />
      </md:IDPSSODescriptor>
    </md:EntityDescriptor>`),
			ValidUntil: aws.Time(timestamp2),
			Tags: []*iam.Tag{
				{Key: aws.String("key1"), Value: aws.String("value1")},
				{Key: aws.String("key2"), Value: aws.String("value2")},
			},
		},
		"arn:aws:iam::1234678:saml-provider/provider2": {
			CreateDate: aws.Time(timestamp2),
			SAMLMetadataDocument: aws.String(`<?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="provider2">
      <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
		<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://posturl.teleport.local" />
		<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://redirecturl.teleport.local" />
      </md:IDPSSODescriptor>
    </md:EntityDescriptor>`),
		},
	}
}

func TestAWSIAMPollOIDCProviders(t *testing.T) {
	const (
		accountID = "12345678"
	)
	var (
		regions = []string{"eu-west-1"}
	)

	timestamp1 := time.Date(2024, time.May, 1, 1, 2, 3, 0, time.UTC)
	timestamp2 := timestamp1.AddDate(1, 0, 0)

	mockedClients := &cloud.TestCloudClients{
		IAM: &mocks.IAMMock{
			OIDCProviders: map[string]*iam.GetOpenIDConnectProviderOutput{
				"arn:aws:iam::1234678:oidc-provider/provider1": {
					ClientIDList: aws.StringSlice([]string{"audience1", "audience2"}),
					CreateDate:   aws.Time(timestamp1),
					Tags: []*iam.Tag{
						{Key: aws.String("key1"), Value: aws.String("value1")},
						{Key: aws.String("key2"), Value: aws.String("value2")},
					},
					ThumbprintList: aws.StringSlice([]string{"thumb1", "thumb2"}),
					Url:            aws.String("https://example.com"),
				},
				"arn:aws:iam::1234678:oidc-provider/provider2": {
					CreateDate: aws.Time(timestamp2),
					Url:        aws.String("https://teleport.local"),
				},
			},
		},
	}

	var (
		errs []error
		mu   sync.Mutex
	)

	collectErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}
	a := &Fetcher{
		Config: Config{
			AccountID:    accountID,
			CloudClients: mockedClients,
			Regions:      regions,
			Integration:  accountID,
		},
		lastResult: &Resources{},
	}
	expected := []*accessgraphv1alpha.AWSOIDCProviderV1{
		{
			Arn:       "arn:aws:iam::1234678:oidc-provider/provider1",
			CreatedAt: timestamppb.New(timestamp1),
			Tags: []*accessgraphv1alpha.AWSTag{
				{Key: "key1", Value: &wrapperspb.StringValue{Value: "value1"}},
				{Key: "key2", Value: &wrapperspb.StringValue{Value: "value2"}},
			},
			AccountId:   accountID,
			ClientIds:   []string{"audience1", "audience2"},
			Thumbprints: []string{"thumb1", "thumb2"},
			Url:         "https://example.com",
		},
		{
			Arn:       "arn:aws:iam::1234678:oidc-provider/provider2",
			CreatedAt: timestamppb.New(timestamp2),
			AccountId: accountID,
			Url:       "https://teleport.local",
		},
	}
	result := &Resources{}
	execFunc := a.pollAWSOIDCProviders(context.Background(), result, collectErr)
	require.NoError(t, execFunc())
	require.Empty(t, errs)
	sortByARN(result.OIDCProviders)
	require.Empty(t, cmp.Diff(
		expected,
		result.OIDCProviders,
		protocmp.Transform(),
		protocmp.IgnoreFields(&accessgraphv1alpha.AWSOIDCProviderV1{}, "last_sync_time"),
	))
}

// sortByARN sorts a slice of resources that have a GetArn() function by the ARN.
func sortByARN[T interface{ GetArn() string }](objs []T) {
	slices.SortFunc(objs, func(t1, t2 T) int {
		return strings.Compare(t1.GetArn(), t2.GetArn())
	})
}
