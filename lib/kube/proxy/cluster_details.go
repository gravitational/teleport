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

package proxy

import (
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
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

	// rwMu is the mutex to protect the kubeCodecs, gvkSupportedResources, and rbacSupportedTypes.
	rwMu sync.RWMutex
	// kubeCodecs is the codec factory for the cluster resources.
	// The codec factory includes the default resources and the namespaced resources
	// that are supported by the cluster.
	// The codec factory is updated periodically to include the latest custom resources
	// that are added to the cluster.
	kubeCodecs serializer.CodecFactory
	// rbacSupportedTypes is the list of supported types for RBAC for the cluster.
	// The list is updated periodically to include the latest custom resources
	// that are added to the cluster.
	rbacSupportedTypes rbacSupportedResources
	// gvkSupportedResources is the list of registered API path resources and their
	// GVK definition.
	gvkSupportedResources gvkSupportedResources
	// isClusterOffline is true if the cluster is offline.
	// An offline cluster will not be able to serve any requests until it comes back online.
	// The cluster is marked as offline if the cluster schema cannot be created
	// and the list of supported types for RBAC cannot be generated.
	isClusterOffline bool

	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// clusterDetailsConfig contains the configuration for creating a proxied cluster.
type clusterDetailsConfig struct {
	// cloudClients is the cloud clients to use for dynamic clusters.
	cloudClients cloud.Clients
	// kubeCreds is the credentials to use for the cluster.
	kubeCreds kubeCreds
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
	// component is the Kubernetes component that serves this cluster.
	component KubeServiceType
}

// newClusterDetails creates a proxied kubeDetails structure given a dynamic cluster.
func newClusterDetails(ctx context.Context, cfg clusterDetailsConfig) (_ *kubeDetails, err error) {
	creds := cfg.kubeCreds
	if creds == nil {
		creds, err = getKubeClusterCredentials(ctx, cfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var dynLabels *labels.Dynamic
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
	var isClusterOffline bool
	// Create the codec factory and the list of supported types for RBAC.
	codecFactory, rbacSupportedTypes, gvkSupportedRes, err := newClusterSchemaBuilder(cfg.log, creds.getKubeClient())
	if err != nil {
		cfg.log.WithError(err).Warn("Failed to create cluster schema. Possibly the cluster is offline.")
		// If the cluster is offline, we will not be able to create the codec factory
		// and the list of supported types for RBAC.
		// We mark the cluster as offline and continue to create the kubeDetails but
		// the offline cluster will not be able to serve any requests until it comes back online.
		isClusterOffline = true
	}

	ctx, cancel := context.WithCancel(ctx)
	k := &kubeDetails{
		kubeCreds:             creds,
		dynamicLabels:         dynLabels,
		kubeCluster:           cfg.cluster,
		kubeCodecs:            codecFactory,
		rbacSupportedTypes:    rbacSupportedTypes,
		cancelFunc:            cancel,
		isClusterOffline:      isClusterOffline,
		gvkSupportedResources: gvkSupportedRes,
	}

	k.wg.Add(1)
	// Start the periodic update of the codec factory and the list of supported types for RBAC.
	go func() {
		defer k.wg.Done()
		ticker := cfg.clock.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.Chan():
				codecFactory, rbacSupportedTypes, gvkSupportedResources, err := newClusterSchemaBuilder(cfg.log, creds.getKubeClient())
				if err != nil {
					cfg.log.WithError(err).Error("Failed to update cluster schema")
					continue
				}

				k.rwMu.Lock()
				k.kubeCodecs = codecFactory
				k.rbacSupportedTypes = rbacSupportedTypes
				k.gvkSupportedResources = gvkSupportedResources
				k.isClusterOffline = false
				k.rwMu.Unlock()
			}
		}
	}()
	return k, nil
}

func (k *kubeDetails) Close() {
	// send a close signal and wait for the close to finish.
	k.cancelFunc()
	k.wg.Wait()
	if k.dynamicLabels != nil {
		k.dynamicLabels.Close()
	}
	// it is safe to call close even for static creds.
	k.kubeCreds.close()
}

// getClusterSupportedResources returns the codec factory and the list of supported types for RBAC.
func (k *kubeDetails) getClusterSupportedResources() (*serializer.CodecFactory, rbacSupportedResources, error) {
	k.rwMu.RLock()
	defer k.rwMu.RUnlock()
	// If the cluster is offline, return an error because we don't have the schema
	// for the cluster.
	if k.isClusterOffline {
		return nil, nil, trace.ConnectionProblem(nil, "kubernetes cluster %q is offline", k.kubeCluster.GetName())
	}
	return &(k.kubeCodecs), k.rbacSupportedTypes, nil
}

// getObjectGVK returns the default GVK (if any) registered for the specified request path.
func (k *kubeDetails) getObjectGVK(resource apiResource) *schema.GroupVersionKind {
	k.rwMu.RLock()
	defer k.rwMu.RUnlock()
	// kube doesn't use core but teleport does.
	if resource.apiGroup == "core" {
		resource.apiGroup = ""
	}
	return k.gvkSupportedResources[gvkSupportedResourcesKey{
		name:     strings.Split(resource.resourceKind, "/")[0],
		apiGroup: resource.apiGroup,
		version:  resource.apiGroupVersion,
	}]
}

// getKubeClusterCredentials generates kube credentials for dynamic clusters.
func getKubeClusterCredentials(ctx context.Context, cfg clusterDetailsConfig) (kubeCreds, error) {
	dynCredsCfg := dynamicCredsConfig{kubeCluster: cfg.cluster, log: cfg.log, checker: cfg.checker, resourceMatchers: cfg.resourceMatchers, clock: cfg.clock, component: cfg.component}
	switch {
	case cfg.cluster.IsKubeconfig():
		return getStaticCredentialsFromKubeconfig(ctx, cfg.component, cfg.cluster, cfg.log, cfg.checker)
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
		opts := []cloud.AWSOptionsFn{
			cloud.WithAmbientCredentials(),
		}
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

		token, exp, err := kubeutils.GenAWSEKSToken(stsClient, cluster.GetAWSConfig().Name, clock)
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

// getStaticCredentialsFromKubeconfig loads a kubeconfig from the cluster and returns the access credentials for the cluster.
// If the config defines multiple contexts, it will pick one (the order is not guaranteed).
func getStaticCredentialsFromKubeconfig(ctx context.Context, component KubeServiceType, cluster types.KubeCluster, log *logrus.Entry, checker servicecfg.ImpersonationPermissionsChecker) (*staticKubeCreds, error) {
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

	creds, err := extractKubeCreds(ctx, component, cluster.GetName(), restConfig, log, checker)
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
