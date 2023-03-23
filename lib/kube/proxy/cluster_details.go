// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// kubeDetails contain the cluster-related details including authentication.
type kubeDetails struct {
	kubeCreds
	// dynamicLabels is the dynamic labels executor for this cluster.
	dynamicLabels *labels.Dynamic
	// kubeCluster is the dynamic kube_cluster or a static generated from kubeconfig and that only has the name populated.
	kubeCluster types.KubeCluster
}

// newClusterDetails creates a proxied kubeDetails structure given a dynamic cluster.
func newClusterDetails(ctx context.Context, cloudClients cloud.Clients, cluster types.KubeCluster, log *logrus.Entry, checker servicecfg.ImpersonationPermissionsChecker) (*kubeDetails, error) {
	var dynLabels *labels.Dynamic

	creds, err := getKubeClusterCredentials(ctx, cloudClients, cluster, log, checker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(cluster.GetDynamicLabels()) > 0 {
		dynLabels, err = labels.NewDynamic(
			ctx,
			&labels.DynamicConfig{
				Labels: cluster.GetDynamicLabels(),
				Log:    log,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dynLabels.Sync()
		go dynLabels.Start()
	}

	return &kubeDetails{
		kubeCreds:     creds,
		dynamicLabels: dynLabels,
		kubeCluster:   cluster,
	}, nil
}

func (k *kubeDetails) Close() {
	if k.dynamicLabels != nil {
		k.dynamicLabels.Close()
	}
	// it is safe to call close even for static creds.
	k.kubeCreds.close()
}

// getKubeClusterCredentials generates kube credentials for dynamic clusters.
func getKubeClusterCredentials(ctx context.Context, cloudClients cloud.Clients, cluster types.KubeCluster, log *logrus.Entry, checker servicecfg.ImpersonationPermissionsChecker) (kubeCreds, error) {
	switch {
	case cluster.IsKubeconfig():
		return getStaticCredentialsFromKubeconfig(ctx, cluster, log, checker)
	case cluster.IsAzure():
		return getAzureCredentials(ctx, cloudClients, cluster, log, checker)
	case cluster.IsAWS():
		return getAWSCredentials(ctx, cloudClients, cluster, log, checker)
	case cluster.IsGCP():
		return getGCPCredentials(ctx, cloudClients, cluster, log, checker)
	default:
		return nil, trace.BadParameter("authentication method provided for cluster %q not supported", cluster.GetName())
	}
}

// getAzureCredentials creates a dynamicCreds that generates and updates the access credentials to a AKS Kubernetes cluster.
func getAzureCredentials(ctx context.Context, cloudClients cloud.Clients, cluster types.KubeCluster, log *logrus.Entry, checker servicecfg.ImpersonationPermissionsChecker) (*dynamicKubeCreds, error) {
	// create a client that returns the credentials for kubeCluster
	client := azureRestConfigClient(cloudClients)

	creds, err := newDynamicKubeCreds(ctx, cluster, log, client, checker)
	return creds, trace.Wrap(err)
}

// azureRestConfigClient creates a dynamicCredsClient that returns credentials to a AKS cluster.
func azureRestConfigClient(cloudClients cloud.Clients) dynamicCredsClient {
	return func(ctx context.Context, cluster types.KubeCluster) (*rest.Config, time.Time, error) {
		aksClient, err := cloudClients.GetAzureKubernetesClient(cluster.GetAzureConfig().SubscriptionID)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}
		cfg, exp, err := aksClient.ClusterCredentials(ctx, azure.ClusterCredentialsConfig{
			ResourceGroup:                   cluster.GetAzureConfig().ResourceGroup,
			ResourceName:                    cluster.GetAzureConfig().ResourceName,
			TenantID:                        cluster.GetAzureConfig().TenantID,
			ImpersonationPermissionsChecker: checkImpersonationPermissions,
		})
		return cfg, exp, trace.Wrap(err)
	}
}

// getAWSCredentials creates a dynamicKubeCreds that generates and updates the access credentials to a EKS kubernetes cluster.
func getAWSCredentials(ctx context.Context, cloudClients cloud.Clients, cluster types.KubeCluster, log *logrus.Entry, checker servicecfg.ImpersonationPermissionsChecker) (*dynamicKubeCreds, error) {
	// create a client that returns the credentials for kubeCluster
	client := getAWSClientRestConfig(cloudClients)
	creds, err := newDynamicKubeCreds(ctx, cluster, log, client, checker)
	return creds, trace.Wrap(err)
}

// getAWSClientRestConfig creates a dynamicCredsClient that generates returns credentials to EKS clusters.
func getAWSClientRestConfig(cloudClients cloud.Clients) dynamicCredsClient {
	return func(ctx context.Context, cluster types.KubeCluster) (*rest.Config, time.Time, error) {
		regionalClient, err := cloudClients.GetAWSEKSClient(cluster.GetAWSConfig().Region)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		eksCfg, err := regionalClient.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
			Name: aws.String(cluster.GetAWSConfig().Name),
		})
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		ca, err := base64.StdEncoding.DecodeString(aws.StringValue(eksCfg.Cluster.CertificateAuthority.Data))
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		apiEndpoint := aws.StringValue(eksCfg.Cluster.Endpoint)
		if len(apiEndpoint) == 0 {
			return nil, time.Time{}, trace.BadParameter("invalid api endpoint for cluster %q", cluster.GetAWSConfig().Name)
		}

		stsClient, err := cloudClients.GetAWSSTSClient(cluster.GetAWSConfig().Region)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		token, exp, err := genAWSToken(stsClient, cluster.GetAWSConfig().Name)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		return &rest.Config{
			Host:        apiEndpoint,
			BearerToken: token,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: ca,
			},
		}, exp, nil
	}
}

// genAWSToken creates an AWS token to access EKS clusters.
// Logic from https://github.com/aws/aws-cli/blob/6c0d168f0b44136fc6175c57c090d4b115437ad1/awscli/customizations/eks/get_token.py#L211-L229
func genAWSToken(stsClient stsiface.STSAPI, clusterID string) (string, time.Time, error) {
	const (
		// The sts GetCallerIdentity request is valid for 15 minutes regardless of this parameters value after it has been
		// signed.
		requestPresignParam = 60
		// The actual token expiration (presigned STS urls are valid for 15 minutes after timestamp in x-amz-date).
		presignedURLExpiration = 15 * time.Minute
		v1Prefix               = "k8s-aws-v1."
		clusterIDHeader        = "x-k8s-aws-id"
	)

	// generate an sts:GetCallerIdentity request and add our custom cluster ID header
	request, _ := stsClient.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	request.HTTPRequest.Header.Add(clusterIDHeader, clusterID)

	// Sign the request.  The expires parameter (sets the x-amz-expires header) is
	// currently ignored by STS, and the token expires 15 minutes after the x-amz-date
	// timestamp regardless.  We set it to 60 seconds for backwards compatibility (the
	// parameter is a required argument to Presign(), and authenticators 0.3.0 and older are expecting a value between
	// 0 and 60 on the server side).
	// https://github.com/aws/aws-sdk-go/issues/2167
	presignedURLString, err := request.Presign(requestPresignParam)
	if err != nil {
		return "", time.Time{}, trace.Wrap(err)
	}

	// Set token expiration to 1 minute before the presigned URL expires for some cushion
	tokenExpiration := time.Now().Local().Add(presignedURLExpiration - 1*time.Minute)
	return v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(presignedURLString)), tokenExpiration, nil
}

// getStaticCredentialsFromKubeconfig loads a kubeconfig from the cluster and returns the access credentials for the cluster.
// If the config defines multiple contexts, it will pick one (the order is not guaranteed).
func getStaticCredentialsFromKubeconfig(ctx context.Context, cluster types.KubeCluster, log *logrus.Entry, checker servicecfg.ImpersonationPermissionsChecker) (*staticKubeCreds, error) {
	config, err := clientcmd.Load(cluster.GetKubeconfig())
	if err != nil {
		return nil, trace.WrapWithMessage(err, "unable to parse kubeconfig for cluster %q", cluster.GetName())
	}
	if len(config.CurrentContext) == 0 && len(config.Contexts) > 0 {
		// select the first context key as default context
		for k := range config.Contexts {
			config.CurrentContext = k
			break
		}
	}
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, nil).ClientConfig()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "unable to create client from kubeconfig for cluster %q", cluster.GetName())
	}

	creds, err := extractKubeCreds(ctx, cluster.GetName(), restConfig, log, checker)
	return creds, trace.Wrap(err)
}

// getGCPCredentials creates a dynamicKubeCreds that generates and updates the access credentials to a GKE kubernetes cluster.
func getGCPCredentials(ctx context.Context, cloudClients cloud.Clients, cluster types.KubeCluster, log *logrus.Entry, checker servicecfg.ImpersonationPermissionsChecker) (*dynamicKubeCreds, error) {
	// create a client that returns the credentials for kubeCluster
	client := gcpRestConfigClient(cloudClients)
	creds, err := newDynamicKubeCreds(ctx, cluster, log, client, checker)
	return creds, trace.Wrap(err)
}

// gcpRestConfigClient creates a dynamicCredsClient that returns credentials to a GKE cluster.
func gcpRestConfigClient(cloudClients cloud.Clients) dynamicCredsClient {
	return func(ctx context.Context, cluster types.KubeCluster) (*rest.Config, time.Time, error) {
		gkeClient, err := cloudClients.GetGCPGKEClient(ctx)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}
		cfg, exp, err := gkeClient.GetClusterRestConfig(ctx,
			gcp.ClusterDetails{
				ProjectID: cluster.GetGCPConfig().ProjectID,
				Location:  cluster.GetGCPConfig().Location,
				Name:      cluster.GetGCPConfig().Name,
			},
		)
		return cfg, exp, trace.Wrap(err)
	}
}
