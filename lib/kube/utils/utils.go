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

package utils

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
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
			if errors.Is(err, rest.ErrNotInCluster) {
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
func EncodeClusterName(clusterName string) string {
	// k is to avoid first letter to be a number
	return "k" + hex.EncodeToString([]byte(clusterName))
}

// ListKubeClustersWithFilters returns a sorted list of unique kubernetes clusters
// registered in p.
func ListKubeClustersWithFilters(ctx context.Context, p client.GetResourcesClient, req proto.ListResourcesRequest) ([]types.KubeCluster, error) {
	req.ResourceType = types.KindKubeServer

	kss, err := client.GetAllResources[types.KubeServer](ctx, p, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return extractAndSortKubeClusters(kss), nil
}

func extractAndSortKubeClusters(kss []types.KubeServer) []types.KubeCluster {
	uniqueClusters := make(map[string]types.KubeCluster)
	for _, ks := range kss {
		uniqueClusters[ks.GetName()] = ks.GetCluster()
	}
	kubeClusters := make([]types.KubeCluster, 0, len(uniqueClusters))
	for _, cluster := range uniqueClusters {
		kubeClusters = append(kubeClusters, cluster)
	}

	sorted := types.KubeClusters(kubeClusters)
	sorted.SortByCustom(types.SortBy{
		Field: types.ResourceMetadataName,
	})

	return []types.KubeCluster(sorted)
}

type Pinger interface {
	Ping(context.Context) (proto.PingResponse, error)
}

// GetKubeAgentVersion returns a version of the Kube agent appropriate for this Teleport cluster. Used for example when deciding version
// for enrolling EKS clusters.
func GetKubeAgentVersion(ctx context.Context, pinger Pinger, clusterFeatures proto.Features, releaseChannels automaticupgrades.Channels) (string, error) {
	pingResponse, err := pinger.Ping(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	agentVersion := pingResponse.ServerVersion

	if clusterFeatures.GetAutomaticUpgrades() && clusterFeatures.GetCloud() {
		defaultVersion, err := releaseChannels.DefaultVersion(ctx)
		if err == nil {
			agentVersion = defaultVersion
		} else if !errors.Is(err, &version.NoNewVersionError{}) {
			return "", trace.Wrap(err)
		}
	}

	return strings.TrimPrefix(agentVersion, "v"), nil
}
