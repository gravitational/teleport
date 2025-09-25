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
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

var kmsTests = []struct {
	name       string
	mockOutput map[string]mocks.KMSOutput
	mockError  error
}{
	{
		name: "single, simple key",
		mockOutput: map[string]mocks.KMSOutput{
			"key1": {
				ARN:          "arn1",
				CreationDate: time.Now(),
			},
		},
	},
	{
		name: "key with tags, policy, multiple regions, aliases, hsm cluster",
		mockOutput: map[string]mocks.KMSOutput{
			"key1": {
				ARN:          "arn1",
				CreationDate: time.Now(),
				Tags:         map[string]string{"key1": "value1"},
				Policy:       "policy1",
				Aliases:      []string{"alias1"},
				MultiType:    types.MultiRegionKeyTypePrimary,
				HSMClusterID: "hsm-cluster1",
			},
		},
	},
	{
		name: "multiple keys",
		mockOutput: map[string]mocks.KMSOutput{
			"key1": {
				ARN:          "arn1",
				CreationDate: time.Now(),
				Tags:         map[string]string{"key1": "value1"},
				Policy:       "policy1",
				Aliases:      []string{"alias1"},
			},
			"key2": {
				ARN:          "arn2",
				CreationDate: time.Now(),
				MultiType:    types.MultiRegionKeyTypeReplica,
				HSMClusterID: "hsm-cluster2",
			},
			"key3": {
				ARN:          "arn3",
				CreationDate: time.Now(),
				HSMClusterID: "hsm-cluster3",
			},
		},
	},
	{
		name:      "error listing keys",
		mockError: errors.New("mock error"),
	},
	{
		name: "multiple key errors",
		mockOutput: map[string]mocks.KMSOutput{
			"key1": {
				ARN:          "arn1",
				CreationDate: time.Now(),
				Tags:         map[string]string{"key1": "value1"},
				Policy:       "policy1",
				Aliases:      []string{"alias1"},

				DescribeKeyErr: errors.New("describeKey"),
			},
			"key2": {
				ARN:          "arn2",
				CreationDate: time.Now(),
				MultiType:    types.MultiRegionKeyTypeReplica,
				HSMClusterID: "hsm-cluster2",

				TagsErr: errors.New("tags"),
			},
			"key3": {
				ARN:          "arn3",
				CreationDate: time.Now(),
				HSMClusterID: "hsm-cluster3",

				AliasesErr: errors.New("aliases"),
			},
			"key4": {
				ARN:          "arn4",
				CreationDate: time.Now(),
				HSMClusterID: "hsm-cluster3",
				Aliases:      []string{"alias1"},

				PolicyErr: errors.New("policy"),
			},
		},
	},
}

func TestPollAWSKMS(t *testing.T) {
	const accountID = "12345678"
	var regions = []string{"us-west-2"}
	fetcherConfig := Config{
		AccountID:         accountID,
		Regions:           regions,
		AWSConfigProvider: &mocks.AWSConfigProvider{},
	}

	for _, tt := range kmsTests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &Fetcher{Config: fetcherConfig}
			kmsClient := &mocks.KMSClient{Keys: tt.mockOutput, ListKeysErr: tt.mockError}
			fetcher.awsClients = fakeAWSClients{kmsClient: kmsClient}
			got := &Resources{}
			var gotErr error
			collectErr := func(err error) { gotErr = err }
			pollFn := fetcher.pollAWSKMSKeys(context.Background(), got, collectErr)
			err := pollFn()
			require.NoError(t, err)
			want, wantErr := kmsMockToProto(kmsClient, accountID, regions[0])
			requireResourceEqual(t, want, got)
			requireAggregateErrorEqual(t, wantErr, gotErr)
		})
	}
}

func requireResourceEqual(t *testing.T, want, got *Resources) {
	tagCmp := func(a, b *pb.AWSTag) bool {
		return a.Key < b.Key
	}
	opts := []cmp.Option{
		protocmp.Transform(),
		protocmp.SortRepeated(tagCmp),
		protocmp.IgnoreFields(&pb.AWSKMSKeyV1{}, "last_sync_time"),
	}
	require.Empty(t, cmp.Diff(want, got, opts...))
}

func requireAggregateErrorEqual(t *testing.T, want, got error) {
	if want == nil {
		require.NoError(t, got)
		return
	}
	require.Error(t, got)
	var wantAggregate, gotAggregate trace.Aggregate
	require.ErrorAs(t, want, &wantAggregate)
	require.ErrorAs(t, got, &gotAggregate)
	require.Len(t, gotAggregate.Errors(), 1)
	if !errors.As(wantAggregate.Errors()[0], &wantAggregate) {
		return
	}
	require.ErrorAs(t, gotAggregate.Errors()[0], &gotAggregate)
	wantLen := len(wantAggregate.Errors())
	require.Len(t, gotAggregate.Errors(), wantLen)
}

func kmsMockToProto(c *mocks.KMSClient, accountID, region string) (*Resources, error) {
	var errs []error
	if c.ListKeysErr != nil {
		return &Resources{}, trace.NewAggregate(c.ListKeysErr)
	}

	keyIDs := slices.Collect(maps.Keys(c.Keys))
	slices.Sort(keyIDs)
	var keys []*pb.AWSKMSKeyV1

	for _, keyID := range keyIDs {
		k, ok := c.Keys[keyID]
		if !ok {
			errs = append(errs, trace.Errorf("key %q not found", keyID))
			continue
		}
		var tags []*pb.AWSTag
		for key, val := range k.Tags {
			tag := &pb.AWSTag{Key: key, Value: wrapperspb.String(val)}
			tags = append(tags, tag)
		}
		key := &pb.AWSKMSKeyV1{
			Arn:                k.ARN,
			CreatedAt:          timestamppb.New(k.CreationDate),
			AccountId:          accountID,
			Region:             region,
			HsmClusterId:       k.HSMClusterID,
			PolicyDocument:     []byte(k.Policy),
			Aliases:            k.Aliases,
			Tags:               tags,
			MultiRegionKeyType: string(k.MultiType),
		}
		if k.DescribeKeyErr != nil {
			key.Arn = fmt.Sprintf("arn:aws:kms:%s:%s:key/%s", region, accountID, keyID)
			key.CreatedAt = nil
		}
		keys = append(keys, key)
		errs = append(errs, k.DescribeKeyErr, k.TagsErr, k.AliasesErr, k.PolicyErr)
	}
	err := trace.NewAggregate(errs...)
	if err != nil {
		err = trace.NewAggregate(err)
	}
	return &Resources{KMSKeys: keys}, err
}
