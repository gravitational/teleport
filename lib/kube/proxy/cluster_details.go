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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

// kubeDetails contain the cluster-related details including authentication.
type kubeDetails struct {
	*kubeCreds
	// dynamicLabels is the dynamic labels executor for this cluster.
	dynamicLabels *labels.Dynamic
	// kubeCluster is the dynamic kube_cluster or a static generated from kubeconfig and that only has the name populated.
	kubeCluster types.KubeCluster
}

// newClusterDetails creates a proxied kubeDetails structure given a dynamic cluster.
func newClusterDetails(ctx context.Context, cluster types.KubeCluster, log *logrus.Entry, checker ImpersonationPermissionsChecker) (*kubeDetails, error) {
	var (
		dynLabels *labels.Dynamic
	)

	creds, err := parseKubeClusterCredentials(ctx, cluster, log, checker)
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
}

// parseKubeClusterCredentials generates kube credentials for dynamic clusters.
// TODO(tigrato): add support for aws and azure logins via token
func parseKubeClusterCredentials(ctx context.Context, cluster types.KubeCluster, log *logrus.Entry, checker ImpersonationPermissionsChecker) (*kubeCreds, error) {
	switch {
	case len(cluster.GetKubeconfig()) > 0:
		return parseKubeConfig(ctx, cluster, log, checker)
	default:
		return nil, trace.BadParameter("authentication method provided for cluster %q not supported", cluster.GetName())
	}
}

// parseKubeConfig loads a kubeconfig from the cluster and returns the access credentials for the cluster.
// If the config defines multiple contexts, it will pick one (the order is not guaranteed).
func parseKubeConfig(ctx context.Context, cluster types.KubeCluster, log *logrus.Entry, checker ImpersonationPermissionsChecker) (*kubeCreds, error) {
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
