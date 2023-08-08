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

package fetchers

import (
	"context"
	"fmt"
	"sync"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// KubeAppsFetcherConfig configures kubeAppFetcher
type KubeAppsFetcherConfig struct {
	// Name of the kubernetes cluster
	ClusterName string
	// KubernetesClient is a client for Kubernetes API
	KubernetesClient kubernetes.Interface
	// FilterLabels are the filter criteria.
	FilterLabels types.Labels
	// Namespaces are the kubernetes namespaces in which to discover services
	Namespaces []string
	// Log is a logger to use
	Log logrus.FieldLogger
	// PI inspects port to find your whether they are HTTP/HTTPS or not.
	protocolChecker services.ProtocolChecker
}

// CheckAndSetDefaults validates and sets the defaults values.
func (k *KubeAppsFetcherConfig) CheckAndSetDefaults() error {
	if k.FilterLabels == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if k.KubernetesClient == nil {
		return trace.BadParameter("missing parameter KubernetesClient")
	}
	if k.Log == nil {
		return trace.BadParameter("missing parameter Log")
	}
	if k.ClusterName == "" {
		return trace.BadParameter("missing parameter ClusterName")
	}
	if k.protocolChecker == nil {
		k.protocolChecker = &noopProtocolChecker{}
	}

	return nil
}

// kubeAppFetcher fetches app resources from Kubernetes services
type kubeAppFetcher struct {
	KubeAppsFetcherConfig
}

// Default implementation, doesn't actually performs HTTP request.
type noopProtocolChecker struct{}

// CheckProtocol for noopProtocolChecker just returns 'tcp'
func (*noopProtocolChecker) CheckProtocol(uri string) string {
	return "tcp"
}

// NewKubeAppsFetcher creates new Kubernetes app fetcher
func NewKubeAppsFetcher(cfg KubeAppsFetcherConfig) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &kubeAppFetcher{
		KubeAppsFetcherConfig: cfg,
	}, nil
}

func isInternalKubeService(s v1.Service) bool {
	return (s.GetNamespace() == "default" && s.GetName() == "kubernetes") ||
		(s.GetNamespace() == "kube-system" && s.GetName() == "kube-dns")
}

func (f *kubeAppFetcher) getServices(ctx context.Context) ([]v1.Service, error) {
	result := []v1.Service{}
	for _, namespace := range f.Namespaces {
		ns := namespace
		if namespace == types.Wildcard {
			ns = ""
		}
		kubeServices, err := f.KubernetesClient.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, s := range kubeServices.Items {
			match, _, err := services.MatchLabels(f.FilterLabels, s.Labels)
			if err != nil {
				return nil, trace.Wrap(err)
			} else if match {
				result = append(result, s)
			} else {
				f.Log.WithField("service_name", s.Name).Debug("Service doesn't match labels.")
			}
		}
	}

	return result, nil
}

// Get fetches Kubernetes apps from the cluster
func (f *kubeAppFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	kubeServices, err := f.getServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Converting service to apps can involve performing a HTTP ping to the service ports to determine protocol.
	// Both services and ports inside services are processed in parallel to minimize time.
	// We also set limit to prevent potential spike load on a cluster in case there are a lot of services.
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(10)

	// Convert services to resources
	var appsMu sync.Mutex
	var apps types.Apps
	for _, service := range kubeServices {
		service := service

		g.Go(func() error {
			// Skip kubernetes own internal services
			if isInternalKubeService(service) {
				return nil
			}

			// Skip service if it has type annotation and it's not 'app'
			for k, v := range service.GetAnnotations() {
				if k == types.DiscoveryTypeLabel && v != services.KubernetesMatchersApp {
					return nil
				}
			}

			serviceApps, err := services.NewApplicationsFromKubeService(service, f.ClusterName, f.protocolChecker)
			if err != nil {
				f.Log.Warnf("Could not get app from Kubernetes service: %v", err)
				return nil
			}

			appsMu.Lock()
			apps = append(apps, serviceApps...)
			appsMu.Unlock()
			return nil
		})
	}

	// We already logged individual errors of converting service to apps, there are never errors returned.
	_ = g.Wait()

	return apps.AsResources(), nil
}

func (f *kubeAppFetcher) ResourceType() string {
	return types.KindApp
}

func (f *kubeAppFetcher) Cloud() string {
	return ""
}

func (f *kubeAppFetcher) String() string {
	return fmt.Sprintf("kubeAppFetcher(Namespaces=%v, Labels=%v)", f.Namespaces, f.FilterLabels)
}
