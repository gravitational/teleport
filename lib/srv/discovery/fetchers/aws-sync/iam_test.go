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
	a := &awsFetcher{
		Config: Config{
			AccountID:    accountID,
			CloudClients: mockedClients,
			Regions:      regions,
			Integration:  accountID,
		},
	}
	expected := []*accessgraphv1alpha.AWSSAMLProviderV1{
		{
			Arn:                  "arn:aws:iam::1234678:saml-provider/provider1",
			CreatedAt:            timestamppb.New(timestamp1),
			ValidUntil:           timestamppb.New(timestamp2),
			SamlMetadataDocument: "<foo></foo>",
			Tags: []*accessgraphv1alpha.AWSTag{
				{Key: "key1", Value: &wrapperspb.StringValue{Value: "value1"}},
				{Key: "key2", Value: &wrapperspb.StringValue{Value: "value2"}},
			},
			AccountId: accountID,
		},
		{
			Arn:                  "arn:aws:iam::1234678:saml-provider/provider2",
			CreatedAt:            timestamppb.New(timestamp2),
			SamlMetadataDocument: "<bar></bar>",
			AccountId:            accountID,
		},
	}
	result := &Resources{}
	execFunc := a.pollAWSSAMLProviders(context.Background(), result, collectErr)
	require.NoError(t, execFunc())
	require.Empty(t, errs)
	require.Empty(t, cmp.Diff(
		expected,
		result.SAMLProviders,
		protocmp.Transform(),
	))
}

func samlProviders(timestamp1, timestamp2 time.Time) map[string]*iam.GetSAMLProviderOutput {
	return map[string]*iam.GetSAMLProviderOutput{
		"arn:aws:iam::1234678:saml-provider/provider1": {
			CreateDate:           aws.Time(timestamp1),
			SAMLMetadataDocument: aws.String("<foo></foo>"),
			ValidUntil:           aws.Time(timestamp2),
			Tags: []*iam.Tag{
				{Key: aws.String("key1"), Value: aws.String("value1")},
				{Key: aws.String("key2"), Value: aws.String("value2")},
			},
		},
		"arn:aws:iam::1234678:saml-provider/provider2": {
			CreateDate:           aws.Time(timestamp2),
			SAMLMetadataDocument: aws.String("<bar></bar>"),
		},
	}
}
