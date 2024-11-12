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

package awsoidc

import (
	"context"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/tags"
)

func TestDeployServiceRequest(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	baseReqFn := func() DeployServiceRequest {
		return DeployServiceRequest{
			TeleportClusterName:     "mycluster",
			Region:                  "r",
			SubnetIDs:               []string{"1"},
			TaskRoleARN:             "arn",
			IntegrationName:         "teleportdev",
			DeploymentMode:          DatabaseServiceDeploymentMode,
			TeleportConfigString:    "config using b64",
			DeploymentJoinTokenName: "discover-aws-oidc-iam-token",
		}
	}

	for _, tt := range []struct {
		name            string
		req             func() DeployServiceRequest
		errCheck        require.ErrorAssertionFunc
		reqWithDefaults DeployServiceRequest
	}{
		{
			name: "no fields",
			req: func() DeployServiceRequest {
				return DeployServiceRequest{}
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing teleport cluster name",
			req: func() DeployServiceRequest {
				r := baseReqFn()
				r.TeleportClusterName = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing region",
			req: func() DeployServiceRequest {
				r := baseReqFn()
				r.Region = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "empty list of subnets",
			req: func() DeployServiceRequest {
				r := baseReqFn()
				r.SubnetIDs = []string{}
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing task role arn",
			req: func() DeployServiceRequest {
				r := baseReqFn()
				r.TaskRoleARN = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing integration name",
			req: func() DeployServiceRequest {
				r := baseReqFn()
				r.IntegrationName = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "invalid deployment mode",
			req: func() DeployServiceRequest {
				r := baseReqFn()
				r.DeploymentMode = "invalid"
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "no deployment mode",
			req: func() DeployServiceRequest {
				r := baseReqFn()
				r.DeploymentMode = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "no teleport service config string",
			req: func() DeployServiceRequest {
				r := baseReqFn()
				r.TeleportConfigString = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name:     "fill defaults",
			req:      baseReqFn,
			errCheck: require.NoError,
			reqWithDefaults: DeployServiceRequest{
				TeleportClusterName:     "mycluster",
				TeleportVersionTag:      teleport.Version,
				Region:                  "r",
				SubnetIDs:               []string{"1"},
				TaskRoleARN:             "arn",
				ClusterName:             stringPointer("mycluster-teleport"),
				ServiceName:             stringPointer("mycluster-teleport-database-service"),
				TaskName:                stringPointer("mycluster-teleport-database-service"),
				DeploymentJoinTokenName: "discover-aws-oidc-iam-token",
				IntegrationName:         "teleportdev",
				ResourceCreationTags: tags.AWSTags{
					"teleport.dev/origin":      "integration_awsoidc",
					"teleport.dev/cluster":     "mycluster",
					"teleport.dev/integration": "teleportdev",
				},
				DeploymentMode:       DatabaseServiceDeploymentMode,
				TeleportConfigString: "config using b64",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.req()
			err := r.CheckAndSetDefaults()
			tt.errCheck(t, err)

			if err != nil {
				return
			}

			require.Empty(t, cmp.Diff(tt.reqWithDefaults, r))
		})
	}
}

func TestNormalizeECSResourceName(t *testing.T) {
	validClusterName := regexp.MustCompile(`^[0-9A-Za-z_\-@:./+]+$`)
	validECSName := regexp.MustCompile(`^[0-9A-Za-z_\-]+$`)
	for _, tt := range []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid",
			input:    "mycluster",
			expected: "mycluster",
		},
		{
			name:     "with dots",
			input:    "mycluster.example",
			expected: "mycluster_example",
		},
		{
			name:     "cloud format",
			input:    "tenant.teleport.sh",
			expected: "tenant_teleport_sh",
		},
		{
			name:     "other special chars",
			input:    "cluster@with:another.host/with+numbers_123",
			expected: "cluster_with_another_host_with_numbers_123",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// ensure test case is valid
			require.True(t, validClusterName.Match([]byte(tt.input)))
			require.True(t, validECSName.Match([]byte(tt.expected)))

			require.Equal(t, tt.expected, normalizeECSResourceName(tt.input))
		})
	}
}

func TestUpsertTask(t *testing.T) {
	ctx := context.Background()

	mockClient := &mockDeployServiceClient{
		clusters:        map[string]*ecstypes.Cluster{},
		taskDefinitions: map[string]*ecstypes.TaskDefinition{},
		services:        map[string]*ecstypes.Service{},
		accountId:       aws.String("123456789012"),
		iamTokenMissing: true,
	}

	expected := []ecstypes.KeyValuePair{
		{
			Name:  aws.String(types.InstallMethodAWSOIDCDeployServiceEnvVar),
			Value: aws.String("true"),
		},
		{
			Name:  aws.String(automaticupgrades.EnvUpgraderVersion),
			Value: aws.String(teleport.Version),
		},
	}

	semVer := *teleport.SemVersion
	semVer.PreRelease = ""
	taskDefinition, err := upsertTask(ctx, mockClient, upsertTaskRequest{TeleportVersionTag: semVer.String()})
	require.NoError(t, err)
	require.Equal(t, expected, taskDefinition.ContainerDefinitions[0].Environment)
}
