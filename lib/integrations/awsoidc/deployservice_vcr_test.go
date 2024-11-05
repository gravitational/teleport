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
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"

	"github.com/gravitational/teleport/api/types"
)

func TestDeployDBService(t *testing.T) {
	ctx := context.Background()

	// To record new fixtures ensure the following:
	// - change recordingMode to recorder.ModeRecordOnce
	recordingMode := recorder.ModeReplayOnly
	// - get a token by
	//   - add `fmt.Println(clientReq.Token)` in `NewDeployServiceClient`
	//   - hosting teleport in a public endpoint and configure the AWS OIDC Integration
	//   - issue a DeployService call and look for the token in the logs
	awsOIDCToken := "x.y.z"

	awsRegion := "us-east-1"
	awsAccountID := "278576220453"
	awsOIDCRoleARN := "arn:aws:iam::" + awsAccountID + ":role/MarcoTestRoleOIDCProvider"
	integrationName := "teleportdev"
	taskRole := "MarcoEC2Role"

	removeKeysRegex, err := regexp.Compile(`(?s)(<AccessKeyId>).*(</AccessKeyId>).*(<SecretAccessKey>).*(</SecretAccessKey>).*(<SessionToken>).*(</SessionToken>)`)
	require.NoError(t, err)
	removeSensitiveHeadersHook := func(i *cassette.Interaction) error {
		i.Request.Headers.Del("Authorization")
		i.Request.Headers.Del("X-Amz-Security-Token")
		i.Request.Form.Del("WebIdentityToken")

		// Requests to STS contain tokens in both HTTP request and response.
		if i.Request.URL == "https://sts.us-east-1.amazonaws.com/" {
			i.Request.Body = ""
			i.Response.Body = removeKeysRegex.ReplaceAllString(i.Response.Body, "${1}x${2}${3}x${4}${5}x${6}")
		}

		return nil
	}

	awsClientReqFunc := func(httpClient *http.Client) *AWSClientRequest {
		return &AWSClientRequest{
			// To record new fixtures you will need a valid token.
			// You can get one by getting the generated token in a real cluster.
			Token:      awsOIDCToken,
			RoleARN:    awsOIDCRoleARN,
			Region:     awsRegion,
			httpClient: httpClient,
		}
	}

	deployServiceReqFunc := func(clusterName string) DeployServiceRequest {
		return DeployServiceRequest{
			Region:    awsRegion,
			AccountID: awsAccountID,
			SubnetIDs: []string{
				"subnet-0b7ab67161173748b",
				"subnet-0dda93c8621eb2e99",
				"subnet-034f17b3f7344e375",
				"subnet-04a07d4721a3c96e0",
				"subnet-0ef025345dd791986",
				"subnet-099632749366c2c56",
			},
			TaskRoleARN:             taskRole,
			TeleportClusterName:     clusterName,
			IntegrationName:         integrationName,
			DeploymentMode:          DatabaseServiceDeploymentMode,
			DeploymentJoinTokenName: "my-iam-join-token",
			TeleportConfigString:    "config using b64",
		}
	}

	mustRecordUsing := func(t *testing.T, name string) *recorder.Recorder {
		r, err := recorder.NewWithOptions(&recorder.Options{
			CassetteName:       name,
			SkipRequestLatency: true,
			Mode:               recordingMode,
		})
		require.NoError(t, err)
		r.AddHook(removeSensitiveHeadersHook, recorder.BeforeSaveHook)
		return r
	}

	rule := &types.TokenRule{
		AWSAccount: "123456789012",
		AWSRegions: []string{awsRegion},
		AWSRole:    taskRole,
	}

	iamJoinToken := &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name: "some-token-name",
		},
		Spec: types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodIAM,
			Roles:      types.SystemRoles{types.RoleDatabase},
			Allow:      []*types.TokenRule{rule},
		},
	}

	tokenStore := mockGetUpsertToken{
		token: iamJoinToken,
	}

	t.Run("nothing exists in aws account", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/emptyaccount")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		resp, err := DeployService(ctx, clt, deployServiceReqFunc("cluster1002"))
		require.NoError(t, err)

		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:cluster/cluster1002-teleport", resp.ClusterARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:service/cluster1002-teleport/cluster1002-teleport-database-service", resp.ServiceARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:task-definition/cluster1002-teleport-database-service:1", resp.TaskDefinitionARN)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/cluster1002-teleport/services/cluster1002-teleport-database-service", resp.ServiceDashboardURL)
	})

	t.Run("recreate everything", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/replace")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		resp, err := DeployService(ctx, clt, deployServiceReqFunc("cluster1002"))
		require.NoError(t, err)

		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:cluster/cluster1002-teleport", resp.ClusterARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:service/cluster1002-teleport/cluster1002-teleport-database-service", resp.ServiceARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:task-definition/cluster1002-teleport-database-service:2", resp.TaskDefinitionARN)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/cluster1002-teleport/services/cluster1002-teleport-database-service", resp.ServiceDashboardURL)
	})

	t.Run("service is being deleted", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/servicedeleted")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		_, err = DeployService(ctx, clt, deployServiceReqFunc("cluster1002"))
		require.ErrorContains(t, err, "ECS Service is draining, please retry in a couple of minutes")
	})

	t.Run("cluster is being deleted", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/clusterdeleted")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		resp, err := DeployService(ctx, clt, deployServiceReqFunc("cluster1002"))
		require.NoError(t, err)

		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:cluster/cluster1002-teleport", resp.ClusterARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:service/cluster1002-teleport/cluster1002-teleport-database-service", resp.ServiceARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:task-definition/cluster1002-teleport-database-service:5", resp.TaskDefinitionARN)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/cluster1002-teleport/services/cluster1002-teleport-database-service", resp.ServiceDashboardURL)
	})

	t.Run("cluster does not have the required capacity provider", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/clustercapacityprovider")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		resp, err := DeployService(ctx, clt, deployServiceReqFunc("cluster1002"))
		require.NoError(t, err)

		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:cluster/cluster1002-teleport", resp.ClusterARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:service/cluster1002-teleport/cluster1002-teleport-database-service", resp.ServiceARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:task-definition/cluster1002-teleport-database-service:9", resp.TaskDefinitionARN)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/cluster1002-teleport/services/cluster1002-teleport-database-service", resp.ServiceDashboardURL)
	})

	t.Run("cluster does not have the ownership tags", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/cluster_without_ownership_tags")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		_, err = DeployService(ctx, clt, deployServiceReqFunc("cluster1002"))
		require.ErrorContains(t, err, `ECS Cluster "cluster1002-teleport" already exists but is not managed by Teleport. Add the following tags to allow Teleport to manage this cluster:`)
	})

	t.Run("service does not have the ownership tags", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/service_without_ownership_tags")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		_, err = DeployService(ctx, clt, deployServiceReqFunc("cluster1002"))
		require.ErrorContains(t, err, `ECS Service "cluster1002-teleport-database-service" already exists but is not managed by Teleport. Add the following tags to allow Teleport to manage this service:`)
	})

	t.Run("cluster name with dots", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/cluster_name_with_dots")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		resp, err := DeployService(ctx, clt, deployServiceReqFunc("tenant-a.teleport.sh"))
		require.NoError(t, err)

		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:cluster/tenant-a_teleport_sh-teleport", resp.ClusterARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:service/tenant-a_teleport_sh-teleport/tenant-a_teleport_sh-teleport-database-service", resp.ServiceARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:task-definition/tenant-a_teleport_sh-teleport-database-service:1", resp.TaskDefinitionARN)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/tenant-a_teleport_sh-teleport/services/tenant-a_teleport_sh-teleport-database-service", resp.ServiceDashboardURL)
	})

	t.Run("deploying without providing an AccountID", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/without_account_id")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		clt, err := NewDeployServiceClient(ctx, awsClientRecorder, &tokenStore)
		require.NoError(t, err)

		deployReq := deployServiceReqFunc("marco-test.teleport.sh")
		deployReq.AccountID = ""
		resp, err := DeployService(ctx, clt, deployReq)
		require.NoError(t, err)

		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:cluster/marco-test_teleport_sh-teleport", resp.ClusterARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:service/marco-test_teleport_sh-teleport/marco-test_teleport_sh-teleport-database-service", resp.ServiceARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:task-definition/marco-test_teleport_sh-teleport-database-service:1", resp.TaskDefinitionARN)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/marco-test_teleport_sh-teleport/services/marco-test_teleport_sh-teleport-database-service", resp.ServiceDashboardURL)
	})
}
