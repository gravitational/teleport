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

package mocks

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type KMSClient struct {
	Keys        map[string]KMSOutput
	ListKeysErr error
}

type KMSOutput struct {
	ARN          string
	Aliases      []string
	CreationDate time.Time
	HSMClusterID string
	MultiType    types.MultiRegionKeyType
	Tags         map[string]string
	Policy       string

	DescribeKeyErr error
	TagsErr        error
	AliasesErr     error
	PolicyErr      error
}

func (c *KMSClient) ListKeys(_ context.Context, input *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	if c.ListKeysErr != nil {
		return nil, c.ListKeysErr
	}
	keyIDs := slices.Collect(maps.Keys(c.Keys))
	slices.Sort(keyIDs)
	keys := make([]types.KeyListEntry, len(keyIDs))
	for i, keyID := range keyIDs {
		keys[i] = types.KeyListEntry{KeyId: &keyID}
	}
	return &kms.ListKeysOutput{Keys: keys}, nil
}

func (c *KMSClient) ListResourceTags(ctx context.Context, input *kms.ListResourceTagsInput, optFns ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
	key, ok := c.Keys[*input.KeyId]
	if !ok {
		return nil, fmt.Errorf("key %q not found to list tags", *input.KeyId)
	}
	if key.TagsErr != nil {
		return nil, key.TagsErr
	}
	tags := make([]types.Tag, 0, len(key.Tags))
	for key, val := range key.Tags {
		tag := types.Tag{
			TagKey:   &key,
			TagValue: &val,
		}
		tags = append(tags, tag)
	}
	return &kms.ListResourceTagsOutput{Tags: tags}, nil
}

func (c *KMSClient) ListAliases(ctx context.Context, input *kms.ListAliasesInput, optFns ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	key, ok := c.Keys[*input.KeyId]
	if !ok {
		return nil, fmt.Errorf("key %q not found to list aliases", *input.KeyId)
	}
	if key.AliasesErr != nil {
		return nil, key.AliasesErr
	}
	aliases := make([]types.AliasListEntry, len(key.Aliases))
	for i, alias := range key.Aliases {
		aliases[i] = types.AliasListEntry{AliasName: &alias}
	}
	return &kms.ListAliasesOutput{Aliases: aliases}, nil
}

func (c *KMSClient) DescribeKey(ctx context.Context, input *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	key, ok := c.Keys[*input.KeyId]
	if !ok {
		return nil, fmt.Errorf("key %q not found to describe", *input.KeyId)
	}
	if key.DescribeKeyErr != nil {
		return nil, key.DescribeKeyErr
	}
	var multiRegionConfig *types.MultiRegionConfiguration
	var hsmClusterID *string
	if key.HSMClusterID != "" {
		hsmClusterID = &key.HSMClusterID
	}
	if key.MultiType != "" {
		multiRegionConfig = &types.MultiRegionConfiguration{
			MultiRegionKeyType: key.MultiType,
		}
	}
	output := &kms.DescribeKeyOutput{
		KeyMetadata: &types.KeyMetadata{
			Arn:                      &key.ARN,
			KeyId:                    input.KeyId,
			CreationDate:             &key.CreationDate,
			CloudHsmClusterId:        hsmClusterID,
			MultiRegionConfiguration: multiRegionConfig,
		},
	}
	return output, nil
}

func (c *KMSClient) GetKeyPolicy(ctx context.Context, input *kms.GetKeyPolicyInput, optFns ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	key, ok := c.Keys[*input.KeyId]
	if !ok {
		return nil, fmt.Errorf("key %q not found to get policy", *input.KeyId)
	}
	if key.PolicyErr != nil {
		return nil, key.PolicyErr
	}
	return &kms.GetKeyPolicyOutput{Policy: &key.Policy}, nil
}
