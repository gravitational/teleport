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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

type kmsClient interface {
	kms.ListKeysAPIClient
	kms.ListResourceTagsAPIClient
	kms.ListAliasesAPIClient
	DescribeKey(ctx context.Context, params *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	GetKeyPolicy(ctx context.Context, params *kms.GetKeyPolicyInput, optFns ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error)
}

// pollAWSKMSKeys is a function that returns a function that fetches
// AWS KMS keys and their inline and attached policies.
func (a *Fetcher) pollAWSKMSKeys(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		result.KMSKeys, err = a.fetchKMSKeys(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch kms"))
		}
		return nil
	}
}

func (a *Fetcher) fetchKMSKeys(ctx context.Context) ([]*pb.AWSKMSKeyV1, error) {
	var keys []*pb.AWSKMSKeyV1
	var errs []error
	var mu sync.Mutex

	collectKeys := func(nextKeys []*pb.AWSKMSKeyV1, nextErrs ...error) {
		mu.Lock()
		defer mu.Unlock()
		if len(nextErrs) > 0 {
			errs = append(errs, nextErrs...)
		}
		if nextKeys != nil {
			keys = append(keys, nextKeys...)
		}
	}

	errGroup := errgroup.Group{}
	// Set the limit to 5 to avoid too many concurrent requests.
	// This is a temporary solution until we have a better way to limit the
	// number of concurrent requests.
	errGroup.SetLimit(5)

	for _, region := range a.Regions {
		errGroup.Go(func() error {
			awsCfg, err := a.AWSConfigProvider.GetConfig(ctx, region, a.getAWSOptions()...)
			if err != nil {
				collectKeys(nil, trace.Wrap(err))
				return nil
			}
			client := a.awsClients.getKMSClient(awsCfg)
			a.collectKMSKeys(ctx, client, collectKeys, region)
			return nil
		})
	}

	err := errGroup.Wait()
	return keys, trace.NewAggregate(append(errs, err)...)
}

type kmsKeyCollector func(newKeys []*pb.AWSKMSKeyV1, errs ...error)

func (a *Fetcher) collectKMSKeys(ctx context.Context, client kmsClient, collectKeys kmsKeyCollector, region string) {
	input := &kms.ListKeysInput{}
	opt := func(opt *kms.ListKeysPaginatorOptions) { opt.StopOnDuplicateToken = true }
	pager := kms.NewListKeysPaginator(client, input, opt)

	var keys []*pb.AWSKMSKeyV1
	var errs []error
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			collectKeys(nil, trace.Wrap(err))
			return
		}
		for _, key := range page.Keys {
			key, err := a.fetchKMSKey(ctx, client, key.KeyId, region)
			if err != nil {
				collectKeys(nil, err)
			}
			if key != nil {
				keys = append(keys, key)
			}
		}
	}
	collectKeys(keys, errs...)
}

func (a *Fetcher) fetchKMSKey(ctx context.Context, client kmsClient, keyID *string, region string) (*pb.AWSKMSKeyV1, error) {
	input := &kms.DescribeKeyInput{KeyId: keyID}
	output, err := client.DescribeKey(ctx, input)
	if err != nil {
		return nil, trace.Wrap(err, "failed to describe KMS key %q", *keyID)
	}
	var errs []error
	result := awsToProtoKMSKey(output, a.AccountID, region)
	result.Tags, err = getTags(ctx, client, keyID)
	if err != nil {
		errs = append(errs, trace.Wrap(err, "cannot fetch tags for KMS key %q", *keyID))
	}
	result.Aliases, err = getAliases(ctx, client, keyID)
	if err != nil {
		errs = append(errs, trace.Wrap(err, "cannot fetch aliases for KMS key %q", *keyID))
	}
	result.PolicyDocument, err = getPolicy(ctx, client, keyID)
	if err != nil {
		errs = append(errs, trace.Wrap(err, "cannot fetch policy for KMS key %q", *keyID))
	}
	if len(errs) > 0 {
		return result, trace.NewAggregate(errs...)
	}
	return result, nil
}

func awsToProtoKMSKey(output *kms.DescribeKeyOutput, accountID, region string) *pb.AWSKMSKeyV1 {
	var multiRegionType pb.MultiRegionKeyType
	cfg := output.KeyMetadata.MultiRegionConfiguration
	switch {
	case cfg == nil:
		multiRegionType = pb.MultiRegionKeyType_MULTI_REGION_KEY_TYPE_NONE // single region
	case cfg.MultiRegionKeyType == types.MultiRegionKeyTypePrimary:
		multiRegionType = pb.MultiRegionKeyType_MULTI_REGION_KEY_TYPE_PRIMARY
	case cfg.MultiRegionKeyType == types.MultiRegionKeyTypeReplica:
		multiRegionType = pb.MultiRegionKeyType_MULTI_REGION_KEY_TYPE_REPLICA
	}
	return &pb.AWSKMSKeyV1{
		Arn:                aws.ToString(output.KeyMetadata.Arn),
		CreatedAt:          awsTimeToProtoTime(output.KeyMetadata.CreationDate),
		Region:             region,
		AccountId:          accountID,
		LastSyncTime:       timestamppb.Now(),
		HsmClusterId:       aws.ToString(output.KeyMetadata.CloudHsmClusterId),
		MultiRegionKeyType: multiRegionType,
	}
}

func getTags(ctx context.Context, client kmsClient, keyID *string) ([]*pb.AWSTag, error) {
	input := &kms.ListResourceTagsInput{KeyId: keyID}
	pager := kms.NewListResourceTagsPaginator(client, input)
	var tags []*pb.AWSTag
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "failed to list tags for key %s", aws.ToString(keyID))
		}
		for _, t := range page.Tags {
			tag := &pb.AWSTag{
				Key:   aws.ToString(t.TagKey),
				Value: strPtrToWrapper(t.TagValue),
			}
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func getAliases(ctx context.Context, client kmsClient, keyID *string) ([]string, error) {
	input := &kms.ListAliasesInput{KeyId: keyID}
	pager := kms.NewListAliasesPaginator(client, input)
	var aliases []string
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "failed to list aliases for key %s", aws.ToString(keyID))
		}
		for _, alias := range page.Aliases {
			aliases = append(aliases, aws.ToString(alias.AliasName))
		}
	}
	return aliases, nil
}

func getPolicy(ctx context.Context, client kmsClient, keyID *string) ([]byte, error) {
	input := &kms.GetKeyPolicyInput{KeyId: keyID}
	output, err := client.GetKeyPolicy(ctx, input)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get key policy for key %s", aws.ToString(keyID))
	}
	return []byte(aws.ToString(output.Policy)), nil
}
