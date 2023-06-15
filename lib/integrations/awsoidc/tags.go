/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awsoidc

import (
	"fmt"
	"strings"

	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/gravitational/teleport/api/types"
)

const (
	// OriginIntegration identifies a resource whose origin is the AWS OIDC Integration
	OriginIntegration = "aws-oidc-integration"

	// ClusterTagKey is the key for the label which identifies the Teleport Cluster's name that created the AWS Resource.
	ClusterTagKey = types.TeleportNamespace + "/cluster"

	// IntegrationTagKey is the key for the label which identifies the Teleport Integration's name that created the AWS Resource.
	IntegrationTagKey = types.TeleportNamespace + "/integration"
)

type awsTags map[string]string

// String converts awsTags into a ',' separated list of k:v
func (d awsTags) String() string {
	tagsString := make([]string, 0, len(d))
	for k, v := range d {
		tagsString = append(tagsString, fmt.Sprintf("%s:%s", k, v))
	}

	return strings.Join(tagsString, ", ")
}

// DefaultResourceCreationTags returns the default tags that should be applied when creating new AWS resources.
// The following tags are returned:
// - teleport.dev/creator_type: teleport
// - teleport.dev/creator: <clusterName>
// - teleport.dev/origin: aws-oidc-integration
// - teleport.dev/integration: <integrationName>
func DefaultResourceCreationTags(clusterName, integrationName string) awsTags {
	return awsTags{
		types.CreatorTypeLabel: "teleport",
		types.CreatorLabel:     clusterName,
		types.OriginLabel:      types.OriginIntegrationAWSOIDC,
		types.IntegrationLabel: integrationName,
	}
}

// ForECS returns the default tags using the expected type for ECS resources: [ecsTypes.Tag]
func (d awsTags) ForECS() []ecsTypes.Tag {
	ecsTags := make([]ecsTypes.Tag, 0, len(d))
	for k, v := range d {
		k, v := k, v
		ecsTags = append(ecsTags, ecsTypes.Tag{
			Key:   &k,
			Value: &v,
		})
	}
	return ecsTags
}

// MatchesECSTags checks if the awsTags are present and have the same value in resourceTags.
func (d awsTags) MatchesECSTags(resourceTags []ecsTypes.Tag) bool {
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
