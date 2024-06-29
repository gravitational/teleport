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
	"crypto/tls"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// KubeAppsFetcherConfig configures KubeAppFetcher
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
	// ProtocolChecker inspects port to find your whether they are HTTP/HTTPS or not.
	ProtocolChecker ProtocolChecker
}

// CheckAndSetDefaults validates and sets the defaults values.
func (k *KubeAppsFetcherConfig) CheckAndSetDefaults() error {
	if k.FilterLabels == nil {
		return trace.BadParameter("missing parameter FilterLabels")
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
	if k.ProtocolChecker == nil {
		k.ProtocolChecker = NewProtoChecker(false)
	}

	return nil
}

// KubeAppFetcher fetches app resources from Kubernetes services
type KubeAppFetcher struct {
	KubeAppsFetcherConfig
}

// NewKubeAppsFetcher creates new Kubernetes app fetcher
func NewKubeAppsFetcher(cfg KubeAppsFetcherConfig) (*KubeAppFetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &KubeAppFetcher{
		KubeAppsFetcherConfig: cfg,
	}, nil
}

func isInternalKubeService(s v1.Service) bool {
	const kubernetesDefaultServiceName = "kubernetes"
	return (s.GetNamespace() == metav1.NamespaceDefault && s.GetName() == kubernetesDefaultServiceName) ||
		s.GetNamespace() == metav1.NamespaceSystem ||
		s.GetNamespace() == metav1.NamespacePublic
}

func (f *KubeAppFetcher) getServices(ctx context.Context) ([]v1.Service, error) {
	var result []v1.Service
	nextToken := ""
	namespaceFilter := func(ns string) bool {
		return slices.Contains(f.Namespaces, types.Wildcard) || slices.Contains(f.Namespaces, ns)
	}
	for {
		// Get all services in the cluster
		// We need to do this in a loop because the API only returns 500 items at a time
		// and we need to paginate through the results.
		kubeServices, err := f.KubernetesClient.CoreV1().Services(v1.NamespaceAll).List(
			ctx,
			metav1.ListOptions{
				Continue: nextToken,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, s := range kubeServices.Items {
			if !namespaceFilter(s.GetNamespace()) || isInternalKubeService(s) {
				// Namespace is not in the list of namespaces to fetch or it's an internal service
				continue
			}
			match, _, err := services.MatchLabels(f.FilterLabels, s.Labels)
			if err != nil {
				return nil, trace.Wrap(err)
			} else if match {
				result = append(result, s)
			} else {
				f.Log.WithField("service_name", s.Name).Debug("Service doesn't match labels.")
			}
		}
		nextToken = kubeServices.Continue
		if nextToken == "" {
			break
		}
	}
	return result, nil
}

const (
	protoHTTPS = "https"
	protoHTTP  = "http"
	protoTCP   = "tcp"
)

// Get fetches Kubernetes apps from the cluster
func (f *KubeAppFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
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
	var (
		appsMu sync.Mutex
		apps   types.Apps
	)
	for _, service := range kubeServices {
		service := service

		// Skip service if it has type annotation and it's not 'app'
		if v, ok := service.GetAnnotations()[types.DiscoveryTypeLabel]; ok && v != types.KubernetesMatchersApp {
			continue
		}

		// If the service is marked with the ignore annotation, skip it.
		if v := service.GetAnnotations()[types.DiscoveryAppIgnore]; v == "true" {
			continue
		}

		g.Go(func() error {
			protocolAnnotation := service.GetAnnotations()[types.DiscoveryProtocolLabel]

			ports, err := getServicePorts(service)
			if err != nil {
				f.Log.WithError(err).Errorf("could not get ports for the service %q", service.GetName())
				return nil
			}

			portProtocols := map[v1.ServicePort]string{}
			for _, port := range ports {
				switch protocolAnnotation {
				case protoHTTPS, protoHTTP, protoTCP:
					portProtocols[port] = protocolAnnotation
				default:
					if p := autoProtocolDetection(services.GetServiceFQDN(service), port, f.ProtocolChecker); p != protoTCP {
						portProtocols[port] = p
					}
				}
			}

			var newApps types.Apps
			for port, portProtocol := range portProtocols {
				if len(portProtocols) == 1 {
					port.Name = ""
				}
				newApp, err := services.NewApplicationFromKubeService(service, f.ClusterName, portProtocol, port)
				if err != nil {
					f.Log.WithError(err).Warnf("Could not get app from a Kubernetes service %q, port %d", service.GetName(), port.Port)
					return nil
				}
				newApps = append(newApps, newApp)
			}
			appsMu.Lock()
			apps = append(apps, newApps...)
			appsMu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	return apps.AsResources(), nil
}

func (f *KubeAppFetcher) ResourceType() string {
	return types.KindApp
}

func (f *KubeAppFetcher) Cloud() string {
	return ""
}

func (f *KubeAppFetcher) FetcherType() string {
	return types.KubernetesMatchersApp
}

func (f *KubeAppFetcher) String() string {
	return fmt.Sprintf("KubeAppFetcher(Namespaces=%v, Labels=%v)", f.Namespaces, f.FilterLabels)
}

// autoProtocolDetection tries to determine port's protocol. It uses heuristics and port HTTP pinging if needed, provided
// by protocol checker. It is used when no explicit annotation for port's protocol was provided.
//   - If port's AppProtocol specifies `http` or `https` we return it
//   - If port's name is `https` or number is 443 we return `https`
//   - If protocol checker is available it will perform HTTP request to the service fqdn trying to find out protocol. If it
//     gives us result `http` or `https` we return it
//   - If port's name is `http` or number is 80 or 8080, we return `http`
func autoProtocolDetection(serviceFQDN string, port v1.ServicePort, pc ProtocolChecker) string {
	if port.AppProtocol != nil {
		switch p := strings.ToLower(*port.AppProtocol); p {
		case protoHTTP, protoHTTPS:
			return p
		}
	}

	if port.Port == 443 || strings.EqualFold(port.Name, protoHTTPS) {
		return protoHTTPS
	}

	if pc != nil {
		result := pc.CheckProtocol(fmt.Sprintf("%s:%d", serviceFQDN, port.Port))
		if result != protoTCP {
			return result
		}
	}

	if port.Port == 80 || port.Port == 8080 || strings.EqualFold(port.Name, protoHTTP) {
		return protoHTTP
	}

	return protoTCP
}

// ProtocolChecker is an interface used to check what protocol uri serves
type ProtocolChecker interface {
	CheckProtocol(uri string) string
}

func getServicePorts(s v1.Service) ([]v1.ServicePort, error) {
	preferredPort := ""
	for k, v := range s.GetAnnotations() {
		if k == types.DiscoveryPortLabel {
			preferredPort = v
		}
	}
	var availablePorts []v1.ServicePort
	for _, p := range s.Spec.Ports {
		// Only supporting TCP ports.
		if p.Protocol != v1.ProtocolTCP {
			continue
		}
		availablePorts = append(availablePorts, p)
		// If preferred port is specified and we found it in available ports, use this one
		if preferredPort != "" && (preferredPort == strconv.Itoa(int(p.Port)) || p.Name == preferredPort) {
			return []v1.ServicePort{p}, nil
		}
	}

	// If preferred port is specified and we're here, it means we couldn't find it in service's ports.
	if preferredPort != "" {
		return nil, trace.BadParameter("specified preferred port %s is absent among available service ports", preferredPort)
	}

	return availablePorts, nil
}

type ProtoChecker struct {
	InsecureSkipVerify bool
	client             *http.Client
}

func NewProtoChecker(insecureSkipVerify bool) *ProtoChecker {
	p := &ProtoChecker{
		InsecureSkipVerify: insecureSkipVerify,
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: insecureSkipVerify,
				},
			},
		},
	}

	return p
}

func (p *ProtoChecker) CheckProtocol(uri string) string {
	if p.client == nil {
		return protoTCP
	}

	resp, err := p.client.Head(fmt.Sprintf("https://%s", uri))
	if err == nil {
		_ = resp.Body.Close()
		return protoHTTPS
	} else if strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") {
		return protoHTTP
	}

	return protoTCP
}
