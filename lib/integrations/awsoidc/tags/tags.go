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

package tags

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/gravitational/teleport/api/types"
)

type AWSTags map[string]string

// String converts AWSTags into a ',' separated list of k:v
func (d AWSTags) String() string {
	tagsString := make([]string, 0, len(d))
	for k, v := range d {
		tagsString = append(tagsString, fmt.Sprintf("%s:%s", k, v))
	}

	return strings.Join(tagsString, ", ")
}

// DefaultResourceCreationTags returns the default tags that should be applied when creating new AWS resources.
// The following tags are returned:
// - teleport.dev/cluster: <clusterName>
// - teleport.dev/origin: aws-oidc-integration
// - teleport.dev/integration: <integrationName>
func DefaultResourceCreationTags(clusterName, integrationName string) AWSTags {
	return AWSTags{
		types.ClusterLabel:     clusterName,
		types.OriginLabel:      types.OriginIntegrationAWSOIDC,
		types.IntegrationLabel: integrationName,
	}
}

// ToECSTags returns the default tags using the expected type for ECS resources: [ecstypes.Tag]
func (d AWSTags) ToECSTags() []ecstypes.Tag {
	ecsTags := make([]ecstypes.Tag, 0, len(d))
	for k, v := range d {
		ecsTags = append(ecsTags, ecstypes.Tag{
			Key:   &k,
			Value: &v,
		})
	}
	slices.SortFunc(ecsTags, func(a, b ecstypes.Tag) int {
		return cmp.Compare(*a.Key, *b.Key)
	})
	return ecsTags
}

// ToEC2Tags the default tags using the expected type for EC2 resources: [ec2types.Tag]
func (d AWSTags) ToEC2Tags() []ec2types.Tag {
	ec2Tags := make([]ec2types.Tag, 0, len(d))
	for k, v := range d {
		ec2Tags = append(ec2Tags, ec2types.Tag{
			Key:   &k,
			Value: &v,
		})
	}
	slices.SortFunc(ec2Tags, func(a, b ec2types.Tag) int {
		return cmp.Compare(*a.Key, *b.Key)
	})
	return ec2Tags
}

// MatchesECSTags checks if the AWSTags are present and have the same value in resourceTags.
func (d AWSTags) MatchesECSTags(resourceTags []ecstypes.Tag) bool {
	resourceTagsMap := make(map[string]string, len(resourceTags))
	for _, tag := range resourceTags {
		resourceTagsMap[*tag.Key] = *tag.Value
	}

	for awsTagKey, awsTagValue := range d {
		resourceTagValue, found := resourceTagsMap[awsTagKey]
		if !found || resourceTagValue != awsTagValue {
			return false
		}
	}

	return true
}

// MatchesIAMTags checks if the AWSTags are present and have the same value in resourceTags.
func (d AWSTags) MatchesIAMTags(resourceTags []iamtypes.Tag) bool {
	resourceTagsMap := make(map[string]string, len(resourceTags))
	for _, tag := range resourceTags {
		resourceTagsMap[*tag.Key] = *tag.Value
	}

	for awsTagKey, awsTagValue := range d {
		resourceTagValue, found := resourceTagsMap[awsTagKey]
		if !found || resourceTagValue != awsTagValue {
			return false
		}
	}

	return true
}

// ToIAMTags returns the default tags using the expected type for IAM resources: [iamtypes.Tag]
func (d AWSTags) ToIAMTags() []iamtypes.Tag {
	iamTags := make([]iamtypes.Tag, 0, len(d))
	for k, v := range d {
		iamTags = append(iamTags, iamtypes.Tag{
			Key:   &k,
			Value: &v,
		})
	}
	slices.SortFunc(iamTags, func(a, b iamtypes.Tag) int {
		return cmp.Compare(*a.Key, *b.Key)
	})
	return iamTags
}

// ToS3Tags returns the default tags using the expected type for S3 resources: [s3types.Tag]
func (d AWSTags) ToS3Tags() []s3types.Tag {
	s3Tags := make([]s3types.Tag, 0, len(d))
	for k, v := range d {
		s3Tags = append(s3Tags, s3types.Tag{
			Key:   &k,
			Value: &v,
		})
	}
	slices.SortFunc(s3Tags, func(a, b s3types.Tag) int {
		return cmp.Compare(*a.Key, *b.Key)
	})
	return s3Tags
}

// ToAthenaTags returns the default tags using the expected type for Athena resources: [athenatypes.Tag]
func (d AWSTags) ToAthenaTags() []athenatypes.Tag {
	athenaTags := make([]athenatypes.Tag, 0, len(d))
	for k, v := range d {
		athenaTags = append(athenaTags, athenatypes.Tag{
			Key:   &k,
			Value: &v,
		})
	}
	slices.SortFunc(athenaTags, func(a, b athenatypes.Tag) int {
		return cmp.Compare(*a.Key, *b.Key)
	})
	return athenaTags
}

// ToSSMTags returns the default tags using the expected type for SSM resources: [ssmtypes.Tag]
func (d AWSTags) ToSSMTags() []ssmtypes.Tag {
	ssmTags := make([]ssmtypes.Tag, 0, len(d))
	for k, v := range d {
		ssmTags = append(ssmTags, ssmtypes.Tag{
			Key:   &k,
			Value: &v,
		})
	}
	slices.SortFunc(ssmTags, func(a, b ssmtypes.Tag) int {
		return cmp.Compare(*a.Key, *b.Key)
	})
	return ssmTags
}

// ToMap returns the default tags using the expected type for other aws resources.
// Eg Glue resources
func (d AWSTags) ToMap() map[string]string {
	return maps.Clone((map[string]string)(d))
}
