/*
Copyright 2020 Gravitational, Inc.

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

package utils

import (
	"context"
	"encoding/hex"
	"sort"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeClient returns instance of client to the kubernetes cluster
// using in-cluster configuration if available and falling back to
// configuration file under configPath otherwise
func GetKubeClient(configPath string) (client *kubernetes.Clientset, config *rest.Config, err error) {
	// if path to kubeconfig was provided, init config from it
	if configPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	} else {
		// otherwise attempt to init as if connecting from cluster
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return client, config, nil
}

// Kubeconfig is a parsed kubeconfig file representation.
type Kubeconfig struct {
	CurrentContext string
	Contexts       map[string]*rest.Config
}

// GetKubeConfig returns kubernetes configuration from configPath file or, by
// default reads in-cluster configuration. If allConfigEntries is set, the
// returned Kubeconfig will contain all contexts from the kubeconfig file;
// otherwise it only contains the current context.
//
// TODO(awly): unit test this
func GetKubeConfig(configPath string, allConfigEntries bool, clusterName string) (*Kubeconfig, error) {
	switch {
	case configPath != "" && clusterName == "":
		loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: configPath}
		cfg, err := loader.Load()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		res := &Kubeconfig{
			CurrentContext: cfg.CurrentContext,
			Contexts:       make(map[string]*rest.Config, len(cfg.Contexts)),
		}
		if !allConfigEntries {
			// Only current-context is requested.
			clientCfg, err := clientcmd.NewNonInteractiveClientConfig(*cfg, cfg.CurrentContext, &clientcmd.ConfigOverrides{}, nil).ClientConfig()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			res.Contexts[cfg.CurrentContext] = clientCfg
			return res, nil
		}
		// All contexts are requested.
		for n := range cfg.Contexts {
			clientCfg, err := clientcmd.NewNonInteractiveClientConfig(*cfg, n, &clientcmd.ConfigOverrides{}, nil).ClientConfig()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			res.Contexts[n] = clientCfg
		}
		return res, nil
	case configPath == "" && clusterName != "":
		cfg, err := rest.InClusterConfig()
		if err != nil {
			if err == rest.ErrNotInCluster {
				return nil, trace.NotFound("not running inside of a Kubernetes pod")
			}
			return nil, trace.Wrap(err)
		}
		return &Kubeconfig{
			CurrentContext: clusterName,
			Contexts:       map[string]*rest.Config{clusterName: cfg},
		}, nil
	case configPath == "" && clusterName == "":
		return nil, trace.NotFound("neither kubeconfig nor cluster name provided")
	case configPath != "" && clusterName != "":
		return nil, trace.BadParameter("only one of configPath or clusterName can be specified")
	}
	panic("unreachable")
}

// EncodeClusterName encodes cluster name for SNI matching
//
// For example:
//
// * Main cluster is main.example.com
// * Remote cluster is remote.example.com
//
// After 'tsh login' the URL of the Kubernetes endpoint of 'remote.example.com'
// when accessed 'via main.example.com' looks like this:
//
// 'k72656d6f74652e6578616d706c652e636f6d0a.main.example.com'
//
// For this to work, users have to add this address in public_addr section of kubernetes service
// to include 'main.example.com' in X509 '*.main.example.com' domain name
//
// where part '72656d6f74652e6578616d706c652e636f6d0a' is a hex encoded remote.example.com
//
// It is hex encoded to allow wildcard matching to work. In DNS wildcard match
// include only one '.'
//
func EncodeClusterName(clusterName string) string {
	// k is to avoid first letter to be a number
	return "k" + hex.EncodeToString([]byte(clusterName))
}

// KubeServicesPresence fetches a list of registered kubernetes services.
// It's a subset of services.Presence.
type KubeServicesPresence interface {
	// GetKubeServices returns a list of registered kubernetes services.
	GetKubeServices(context.Context) ([]types.Server, error)
}

// KubeClusterNames returns a sorted list of unique kubernetes clusters
// registered in p.
func KubeClusterNames(ctx context.Context, p KubeServicesPresence) ([]string, error) {
	kss, err := p.GetKubeServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeClusters := make(map[string]struct{})
	for _, ks := range kss {
		for _, kc := range ks.GetKubernetesClusters() {
			kubeClusters[kc.Name] = struct{}{}
		}
	}
	kubeClusterNames := make([]string, 0, len(kubeClusters))
	for n := range kubeClusters {
		kubeClusterNames = append(kubeClusterNames, n)
	}
	sort.Strings(kubeClusterNames)
	return kubeClusterNames, nil
}

// CheckOrSetKubeCluster validates kubeClusterName if it's set, or a sane
// default based on registered clusters.
//
// If no clusters are registered, a NotFound error is returned.
func CheckOrSetKubeCluster(ctx context.Context, p KubeServicesPresence, kubeClusterName, teleportClusterName string) (string, error) {
	kubeClusterNames, err := KubeClusterNames(ctx, p)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if kubeClusterName != "" {
		if !utils.SliceContainsStr(kubeClusterNames, kubeClusterName) {
			return "", trace.BadParameter("kubernetes cluster %q is not registered in this teleport cluster; you can list registered kubernetes clusters using 'tsh kube ls'", kubeClusterName)
		}
		return kubeClusterName, nil
	}
	// Default is the cluster with a name matching the Teleport cluster
	// name (for backwards-compatibility with pre-5.0 behavior) or the
	// first name alphabetically.
	if len(kubeClusterNames) == 0 {
		return "", trace.NotFound("no kubernetes clusters registered")
	}
	if utils.SliceContainsStr(kubeClusterNames, teleportClusterName) {
		return teleportClusterName, nil
	}
	return kubeClusterNames[0], nil
}
