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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/require"
)

func TestDefaultTags(t *testing.T) {
	clusterName := "mycluster"
	integrationName := "myawsaccount"
	d := DefaultResourceCreationTags(clusterName, integrationName)

	expectedTags := awsTags{
		"teleport.dev/cluster":     "mycluster",
		"teleport.dev/integration": "myawsaccount",
		"teleport.dev/origin":      "integration_awsoidc",
	}
	require.Equal(t, expectedTags, d)

	t.Run("iam tags", func(t *testing.T) {
		expectedIAMTags := []iamTypes.Tag{
			{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
			{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
			{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
		}
		require.ElementsMatch(t, expectedIAMTags, d.ToIAMTags())
	})

	t.Run("ecs tags", func(t *testing.T) {
		expectedECSTags := []ecsTypes.Tag{
			{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
			{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
			{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
		}
		require.ElementsMatch(t, expectedECSTags, d.ToECSTags())
	})

	t.Run("resource is teleport managed", func(t *testing.T) {
		t.Run("all tags match", func(t *testing.T) {
			awsResourceTags := []ecsTypes.Tag{
				{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
				{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
				{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
			}
			require.True(t, d.MatchesECSTags(awsResourceTags), "resource was wrongly detected as not Teleport managed")
		})
		t.Run("extra tags in aws resource", func(t *testing.T) {
			awsResourceTags := []ecsTypes.Tag{
				{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
				{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
				{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
				{Key: aws.String("unrelated"), Value: aws.String("true")},
			}
			require.True(t, d.MatchesECSTags(awsResourceTags), "resource was wrongly detected as not Teleport managed")
		})
		t.Run("missing one of the labels should return false", func(t *testing.T) {
			awsResourceTags := []ecsTypes.Tag{
				{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
				{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
			}
			require.False(t, d.MatchesECSTags(awsResourceTags), "resource was wrongly detected as Teleport managed")
		})
		t.Run("one of the labels has a different value, should return false", func(t *testing.T) {
			awsResourceTags := []ecsTypes.Tag{
				{Key: aws.String("teleport.dev/cluster"), Value: aws.String("another-cluster")},
				{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
				{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
			}
			require.False(t, d.MatchesECSTags(awsResourceTags), "resource was wrongly detected as Teleport managed")
		})
	})
}
