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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/require"
)

func TestDefaultTags(t *testing.T) {
	clusterName := "mycluster"
	integrationName := "myawsaccount"
	origin := "integration_awsoidc"
	d := DefaultResourceCreationTags(clusterName, integrationName, origin)

	expectedTags := AWSTags{
		"teleport.dev/cluster":     "mycluster",
		"teleport.dev/integration": "myawsaccount",
		"teleport.dev/origin":      "integration_awsoidc",
	}
	require.Equal(t, expectedTags, d)

	t.Run("iam tags", func(t *testing.T) {
		expectedIAMTags := []iamtypes.Tag{
			{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
			{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
			{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
		}
		require.ElementsMatch(t, expectedIAMTags, d.ToIAMTags())
	})

	t.Run("ecs tags", func(t *testing.T) {
		expectedECSTags := []ecstypes.Tag{
			{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
			{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
			{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
		}
		require.ElementsMatch(t, expectedECSTags, d.ToECSTags())
	})

	t.Run("ec2 tags", func(t *testing.T) {
		expectedEC2Tags := []ec2types.Tag{
			{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
			{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
			{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
		}
		require.ElementsMatch(t, expectedEC2Tags, d.ToEC2Tags())
	})

	t.Run("ssm tags", func(t *testing.T) {
		expectedTags := []ssmtypes.Tag{
			{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
			{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
			{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
		}
		require.ElementsMatch(t, expectedTags, d.ToSSMTags())
	})

	t.Run("roles anywhere tags", func(t *testing.T) {
		expectedTags := []ratypes.Tag{
			{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
			{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
			{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
		}
		require.ElementsMatch(t, expectedTags, d.ToRolesAnywhereTags())
	})

	t.Run("resource is teleport managed", func(t *testing.T) {
		t.Run("ECS Tags", func(t *testing.T) {
			t.Run("all tags match", func(t *testing.T) {
				awsResourceTags := []ecstypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
				}
				require.True(t, d.MatchesECSTags(awsResourceTags), "resource was wrongly detected as not Teleport managed")
			})
			t.Run("extra tags in aws resource", func(t *testing.T) {
				awsResourceTags := []ecstypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
					{Key: aws.String("unrelated"), Value: aws.String("true")},
				}
				require.True(t, d.MatchesECSTags(awsResourceTags), "resource was wrongly detected as not Teleport managed")
			})
			t.Run("missing one of the labels should return false", func(t *testing.T) {
				awsResourceTags := []ecstypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
				}
				require.False(t, d.MatchesECSTags(awsResourceTags), "resource was wrongly detected as Teleport managed")
			})
			t.Run("one of the labels has a different value, should return false", func(t *testing.T) {
				awsResourceTags := []ecstypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("another-cluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
				}
				require.False(t, d.MatchesECSTags(awsResourceTags), "resource was wrongly detected as Teleport managed")
			})
		})
		t.Run("IAM Tags", func(t *testing.T) {
			t.Run("all tags match", func(t *testing.T) {
				awsResourceTags := []iamtypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
				}
				require.True(t, d.MatchesIAMTags(awsResourceTags), "resource was wrongly detected as not Teleport managed")
			})
			t.Run("extra tags in aws resource", func(t *testing.T) {
				awsResourceTags := []iamtypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
					{Key: aws.String("unrelated"), Value: aws.String("true")},
				}
				require.True(t, d.MatchesIAMTags(awsResourceTags), "resource was wrongly detected as not Teleport managed")
			})
			t.Run("missing one of the labels should return false", func(t *testing.T) {
				awsResourceTags := []iamtypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
				}
				require.False(t, d.MatchesIAMTags(awsResourceTags), "resource was wrongly detected as Teleport managed")
			})
			t.Run("one of the labels has a different value, should return false", func(t *testing.T) {
				awsResourceTags := []iamtypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("another-cluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myawsaccount")},
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
				}
				require.False(t, d.MatchesIAMTags(awsResourceTags), "resource was wrongly detected as Teleport managed")
			})
		})
	})
}
