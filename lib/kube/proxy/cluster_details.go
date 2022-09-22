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
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
func newClusterDetails(ctx context.Context, cloudClients cloud.Clients, cluster types.KubeCluster, log *logrus.Entry, checker ImpersonationPermissionsChecker) (*kubeDetails, error) {
	var (
		dynLabels *labels.Dynamic
	)

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
// TODO(tigrato): add support for aws logins via token
func getKubeClusterCredentials(ctx context.Context, cloudClients cloud.Clients, cluster types.KubeCluster, log *logrus.Entry, checker ImpersonationPermissionsChecker) (kubeCreds, error) {
	switch {
	case cluster.IsKubeconfig():
		return getStaticCredentialsFromKubeconfig(ctx, cluster, log, checker)
	case cluster.IsAzure():
		return getAzureCredentials(ctx, cloudClients, cluster, log, checker)
	default:
		return nil, trace.BadParameter("authentication method provided for cluster %q not supported", cluster.GetName())
	}
}

// getAzureCredentials creates a dynamicCreds that generates and updates the access credentials to a kubernetes cluster.
func getAzureCredentials(ctx context.Context, cloudClients cloud.Clients, cluster types.KubeCluster, log *logrus.Entry, checker ImpersonationPermissionsChecker) (*dynamicCreds, error) {
	// create a client that returns the credentials for kubeCluster
	client := azureRestConfigClient(cloudClients)

	creds, err := newDynamicCreds(ctx, cluster, log, client, checker)
	return creds, trace.Wrap(err)
}

// azureRestConfigClient creates a dynamicCredsClient that generates returns credentials to AKS clusters.
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

// getStaticCredentialsFromKubeconfig loads a kubeconfig from the cluster and returns the access credentials for the cluster.
// If the config defines multiple contexts, it will pick one (the order is not guaranteed).
func getStaticCredentialsFromKubeconfig(ctx context.Context, cluster types.KubeCluster, log *logrus.Entry, checker ImpersonationPermissionsChecker) (*staticKubeCreds, error) {
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
