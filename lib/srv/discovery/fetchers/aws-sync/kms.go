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

package aws_sync

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
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

// pollAWSKMSKeys returns a function that fetches AWS KMS keys, their aliases,
// tags, and their inline key policy.
func (a *Fetcher) pollAWSKMSKeys(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		result.KMSKeys, err = a.fetchKMSKeys(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch KMS keys"))
		}
		return nil
	}
}

// fetchKMSKeys fetches AWS KMS keys, their aliases, tags, and their inline key
// policy. Up to five regions are fetched concurrently.
func (a *Fetcher) fetchKMSKeys(ctx context.Context) ([]*pb.AWSKMSKeyV1, error) {
	var keys []*pb.AWSKMSKeyV1
	var errs []error
	var mu sync.Mutex

	collectKeys := func(nextKeys []*pb.AWSKMSKeyV1, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = append(errs, err)
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
			keys, err := a.fetchKMSKeysForRegion(ctx, client, region)
			collectKeys(keys, err)
			return nil
		})
	}

	err := errGroup.Wait()
	return keys, trace.NewAggregate(append(errs, err)...)
}

// fetchKMSKeysForRegion fetches all AWS KMS keys for a given region and
// converts them to the Teleport protobuf representation. It is lenient with
// errors on individual keys in order to continue fetching other keys. All
// errors encountered are aggregated into a single error, except for pager
// errors which cause an immediate return to avoid an endless loop.
func (a *Fetcher) fetchKMSKeysForRegion(ctx context.Context, client kmsClient, region string) ([]*pb.AWSKMSKeyV1, error) {
	input := &kms.ListKeysInput{}
	opt := func(opt *kms.ListKeysPaginatorOptions) { opt.StopOnDuplicateToken = true }
	pager := kms.NewListKeysPaginator(client, input, opt)

	var keys []*pb.AWSKMSKeyV1
	var errs []error
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, key := range page.Keys {
			key, err := a.fetchKMSKey(ctx, client, aws.ToString(key.KeyId), region)
			if err != nil {
				errs = append(errs, err)
			}
			if key != nil {
				keys = append(keys, key)
			}
		}
	}
	if len(errs) > 0 {
		return keys, trace.NewAggregate(errs...)
	}
	return keys, nil
}

// fetchKMSKey fetches a single AWS KMS key and converts it to the Teleport
// protobuf representation. It is lenient with errors on subqueries to fetch
// tags, aliases and policies and aggregates them into a single error. This is
// useful if permissions don't allow any of the subqueries.
func (a *Fetcher) fetchKMSKey(ctx context.Context, client kmsClient, keyID string, region string) (*pb.AWSKMSKeyV1, error) {
	input := &kms.DescribeKeyInput{KeyId: &keyID}
	output, err := client.DescribeKey(ctx, input)
	if err != nil {
		return nil, trace.Wrap(err, "failed to describe KMS key %q", keyID)
	}
	var errs []error
	result := awsToProtoKMSKey(output, a.AccountID, region)
	result.Tags, err = getTags(ctx, client, keyID)
	if err != nil {
		errs = append(errs, trace.Wrap(err, "cannot fetch tags for KMS key %q", keyID))
	}
	result.Aliases, err = getAliases(ctx, client, keyID)
	if err != nil {
		errs = append(errs, trace.Wrap(err, "cannot fetch aliases for KMS key %q", keyID))
	}
	result.PolicyDocument, err = getPolicy(ctx, client, keyID)
	if err != nil {
		errs = append(errs, trace.Wrap(err, "cannot fetch policy for KMS key %q", keyID))
	}
	if len(errs) > 0 {
		return result, trace.NewAggregate(errs...)
	}
	return result, nil
}

// awsToProtoKMSKey converts an AWS KMS key as represented in the AWS client
// library to the Teleport protobuf representation.
func awsToProtoKMSKey(output *kms.DescribeKeyOutput, accountID, region string) *pb.AWSKMSKeyV1 {
	var multiRegionType string
	if cfg := output.KeyMetadata.MultiRegionConfiguration; cfg != nil {
		multiRegionType = string(cfg.MultiRegionKeyType)
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

// getTags fetches tags for a KMS key. Potentially access rights to tags differ
// to the key access rights as tags are sensitive when used for access control
// via ABAC.
func getTags(ctx context.Context, client kmsClient, keyID string) ([]*pb.AWSTag, error) {
	input := &kms.ListResourceTagsInput{KeyId: &keyID}
	pager := kms.NewListResourceTagsPaginator(client, input)
	var tags []*pb.AWSTag
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "failed to list tags for key %s", keyID)
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

// getAliases fetches aliases for a KMS key. Potentially access rights to
// aliases differ to the key access rights as aliases are sensitive when used
// for access control via ABAC.
func getAliases(ctx context.Context, client kmsClient, keyID string) ([]string, error) {
	input := &kms.ListAliasesInput{KeyId: &keyID}
	pager := kms.NewListAliasesPaginator(client, input)
	var aliases []string
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "failed to list aliases for key %s", keyID)
		}
		for _, alias := range page.Aliases {
			aliases = append(aliases, aws.ToString(alias.AliasName))
		}
	}
	return aliases, nil
}

// getPolicy fetches the attached key policy for a KMS key. There is always
// exactly one key policy per KMS key called default.
func getPolicy(ctx context.Context, client kmsClient, keyID string) ([]byte, error) {
	input := &kms.GetKeyPolicyInput{KeyId: &keyID}
	output, err := client.GetKeyPolicy(ctx, input)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get key policy for key %s", keyID)
	}
	return []byte(aws.ToString(output.Policy)), nil
}
