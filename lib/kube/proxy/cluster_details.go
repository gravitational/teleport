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
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

// kubeDetails contain the cluster-related details including authentication.
type kubeDetails struct {
	kubeCreds
	// dynamicLabels is the dynamic labels executor for this cluster.
	dynamicLabels *labels.Dynamic
	// kubeCluster is the dynamic kube_cluster or a static generated from kubeconfig and that only has the name populated.
	kubeCluster types.KubeCluster
}

// clusterDetailsConfig contains the configuration for creating a proxied cluster.
type clusterDetailsConfig struct {
	// cloudClients is the cloud clients to use for dynamic clusters.
	cloudClients cloud.Clients
	// cluster is the cluster to create a proxied cluster for.
	cluster types.KubeCluster
	// log is the logger to use.
	log *logrus.Entry
	// checker is the permissions checker to use.
	checker servicecfg.ImpersonationPermissionsChecker
	// resourceMatchers is the list of resource matchers to match the cluster against
	// to determine if we should assume the role or not for AWS.
	resourceMatchers []services.ResourceMatcher
	// clock is the clock to use.
	clock clockwork.Clock
}

// newClusterDetails creates a proxied kubeDetails structure given a dynamic cluster.
func newClusterDetails(ctx context.Context, cfg clusterDetailsConfig) (*kubeDetails, error) {
	var dynLabels *labels.Dynamic

	creds, err := getKubeClusterCredentials(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(cfg.cluster.GetDynamicLabels()) > 0 {
		dynLabels, err = labels.NewDynamic(
			ctx,
			&labels.DynamicConfig{
				Labels: cfg.cluster.GetDynamicLabels(),
				Log:    cfg.log,
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
		kubeCluster:   cfg.cluster,
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
func getKubeClusterCredentials(ctx context.Context, cfg clusterDetailsConfig) (kubeCreds, error) {
	dynCredsCfg := dynamicCredsConfig{kubeCluster: cfg.cluster, log: cfg.log, checker: cfg.checker, resourceMatchers: cfg.resourceMatchers, clock: cfg.clock}
	switch {
	case cfg.cluster.IsKubeconfig():
		return getStaticCredentialsFromKubeconfig(ctx, cfg.cluster, cfg.log, cfg.checker)
	case cfg.cluster.IsAzure():
		return getAzureCredentials(ctx, cfg.cloudClients, dynCredsCfg)
	case cfg.cluster.IsAWS():
		return getAWSCredentials(ctx, cfg.cloudClients, dynCredsCfg)
	case cfg.cluster.IsGCP():
		return getGCPCredentials(ctx, cfg.cloudClients, dynCredsCfg)
	default:
		return nil, trace.BadParameter("authentication method provided for cluster %q not supported", cfg.cluster.GetName())
	}
}

// getAzureCredentials creates a dynamicCreds that generates and updates the access credentials to a AKS Kubernetes cluster.
func getAzureCredentials(ctx context.Context, cloudClients cloud.Clients, cfg dynamicCredsConfig) (*dynamicKubeCreds, error) {
	// create a client that returns the credentials for kubeCluster
	cfg.client = azureRestConfigClient(cloudClients)

	creds, err := newDynamicKubeCreds(
		ctx,
		cfg,
	)
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
func getAWSCredentials(ctx context.Context, cloudClients cloud.Clients, cfg dynamicCredsConfig) (*dynamicKubeCreds, error) {
	// create a client that returns the credentials for kubeCluster
	cfg.client = getAWSClientRestConfig(cloudClients, cfg.clock, cfg.resourceMatchers)
	creds, err := newDynamicKubeCreds(ctx, cfg)
	return creds, trace.Wrap(err)
}

// getAWSResourceMatcherToCluster returns the AWS assume role ARN and external ID for the cluster that matches the kubeCluster.
// If no match is found, nil is returned, which means that we should not attempt to assume a role.
func getAWSResourceMatcherToCluster(kubeCluster types.KubeCluster, resourceMatchers []services.ResourceMatcher) *services.ResourceMatcherAWS {
	if !kubeCluster.IsAWS() {
		return nil
	}
	for _, matcher := range resourceMatchers {
		if len(matcher.Labels) == 0 || matcher.AWS.AssumeRoleARN == "" {
			continue
		}
		if match, _, _ := services.MatchLabels(matcher.Labels, kubeCluster.GetAllLabels()); !match {
			continue
		}

		return &(matcher.AWS)
	}
	return nil
}

// getAWSClientRestConfig creates a dynamicCredsClient that generates returns credentials to EKS clusters.
func getAWSClientRestConfig(cloudClients cloud.Clients, clock clockwork.Clock, resourceMatchers []services.ResourceMatcher) dynamicCredsClient {
	return func(ctx context.Context, cluster types.KubeCluster) (*rest.Config, time.Time, error) {
		region := cluster.GetAWSConfig().Region
		var opts []cloud.AWSAssumeRoleOptionFn
		if awsAssume := getAWSResourceMatcherToCluster(cluster, resourceMatchers); awsAssume != nil {
			opts = append(opts, cloud.WithAssumeRole(awsAssume.AssumeRoleARN, awsAssume.ExternalID))
		}
		regionalClient, err := cloudClients.GetAWSEKSClient(ctx, region, opts...)
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

		stsClient, err := cloudClients.GetAWSSTSClient(ctx, region, opts...)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		token, exp, err := genAWSToken(stsClient, cluster.GetAWSConfig().Name, clock)
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
func genAWSToken(stsClient stsiface.STSAPI, clusterID string, clock clockwork.Clock) (string, time.Time, error) {
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
	tokenExpiration := clock.Now().Add(presignedURLExpiration - 1*time.Minute)
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
func getGCPCredentials(ctx context.Context, cloudClients cloud.Clients, cfg dynamicCredsConfig) (*dynamicKubeCreds, error) {
	// create a client that returns the credentials for kubeCluster
	cfg.client = gcpRestConfigClient(cloudClients)
	creds, err := newDynamicKubeCreds(ctx, cfg)
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
