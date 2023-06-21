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
	awsOIDCRoleARN := "arn:aws:iam::278576220453:role/MarcoTestRoleOIDCProvider"
	integrationName := "teleportdev"

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
			Token:           awsOIDCToken,
			RoleARN:         awsOIDCRoleARN,
			Region:          awsRegion,
			IntegrationName: integrationName,
			httpClient:      httpClient,
		}
	}

	deployServiceReqFunc := func(clusterName string) DeployServiceRequest {
		return DeployServiceRequest{
			Region: awsRegion,
			SubnetIDs: []string{
				"subnet-0b7ab67161173748b",
				"subnet-0dda93c8621eb2e99",
				"subnet-034f17b3f7344e375",
				"subnet-04a07d4721a3c96e0",
				"subnet-0ef025345dd791986",
				"subnet-099632749366c2c56",
			},
			TaskRoleARN:         "MarcoEC2Role",
			TeleportClusterName: clusterName,
			IntegrationName:     "teleportdev",
			DeploymentMode:      DatabaseServiceDeploymentMode,
			ProxyServerHostPort: "marcodinis.teleportdemo.net:443",
			DatabaseResourceMatcherLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
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

	t.Run("nothing exists in aws account", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/emptyaccount")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		ecsClient, err := newECSClient(ctx, awsClientRecorder)
		require.NoError(t, err)

		resp, err := DeployService(ctx, ecsClient, deployServiceReqFunc("cluster1002"))
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
		ecsClient, err := newECSClient(ctx, awsClientRecorder)
		require.NoError(t, err)

		resp, err := DeployService(ctx, ecsClient, deployServiceReqFunc("cluster1002"))
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
		ecsClient, err := newECSClient(ctx, awsClientRecorder)
		require.NoError(t, err)

		_, err = DeployService(ctx, ecsClient, deployServiceReqFunc("cluster1002"))
		require.ErrorContains(t, err, "ECS Service is draining, please retry in a couple of minutes")
	})

	t.Run("cluster is being deleted", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/clusterdeleted")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		ecsClient, err := newECSClient(ctx, awsClientRecorder)
		require.NoError(t, err)

		resp, err := DeployService(ctx, ecsClient, deployServiceReqFunc("cluster1002"))
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
		ecsClient, err := newECSClient(ctx, awsClientRecorder)
		require.NoError(t, err)

		resp, err := DeployService(ctx, ecsClient, deployServiceReqFunc("cluster1002"))
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
		ecsClient, err := newECSClient(ctx, awsClientRecorder)
		require.NoError(t, err)

		_, err = DeployService(ctx, ecsClient, deployServiceReqFunc("cluster1002"))
		require.ErrorContains(t, err, `ECS Cluster "cluster1002-teleport" already exists but is not managed by Teleport. Add the following tags to allow Teleport to manage this cluster:`)
	})

	t.Run("service does not have the ownership tags", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/service_without_ownership_tags")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		ecsClient, err := newECSClient(ctx, awsClientRecorder)
		require.NoError(t, err)

		_, err = DeployService(ctx, ecsClient, deployServiceReqFunc("cluster1002"))
		require.ErrorContains(t, err, `ECS Service "cluster1002-teleport-database-service" already exists but is not managed by Teleport. Add the following tags to allow Teleport to manage this service:`)
	})

	t.Run("cluster name with dots", func(t *testing.T) {
		r := mustRecordUsing(t, "fixtures/cluster_name_with_dots")
		defer r.Stop()

		awsClientRecorder := awsClientReqFunc(r.GetDefaultClient())
		ecsClient, err := newECSClient(ctx, awsClientRecorder)
		require.NoError(t, err)

		resp, err := DeployService(ctx, ecsClient, deployServiceReqFunc("tenant-a.teleport.sh"))
		require.NoError(t, err)

		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:cluster/tenant-a_teleport_sh-teleport", resp.ClusterARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:service/tenant-a_teleport_sh-teleport/tenant-a_teleport_sh-teleport-database-service", resp.ServiceARN)
		require.Equal(t, "arn:aws:ecs:us-east-1:278576220453:task-definition/tenant-a_teleport_sh-teleport-database-service:1", resp.TaskDefinitionARN)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/tenant-a_teleport_sh-teleport/services/tenant-a_teleport_sh-teleport-database-service", resp.ServiceDashboardURL)
	})
}
