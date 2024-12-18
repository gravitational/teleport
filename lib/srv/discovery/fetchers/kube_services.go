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

package fetchers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
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
	Logger *slog.Logger
	// ProtocolChecker inspects port to find your whether they are HTTP/HTTPS or not.
	ProtocolChecker ProtocolChecker
	// DiscoveryConfigName is the name of the discovery config which originated the resource.
	DiscoveryConfigName string
}

// CheckAndSetDefaults validates and sets the defaults values.
func (k *KubeAppsFetcherConfig) CheckAndSetDefaults() error {
	if k.FilterLabels == nil {
		return trace.BadParameter("missing parameter FilterLabels")
	}
	if k.KubernetesClient == nil {
		return trace.BadParameter("missing parameter KubernetesClient")
	}
	if k.Logger == nil {
		return trace.BadParameter("missing parameter Logger")
	}
	if k.ClusterName == "" {
		return trace.BadParameter("missing parameter ClusterName")
	}
	if k.ProtocolChecker == nil {
		k.ProtocolChecker = NewProtoChecker()
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

func (f *KubeAppFetcher) getServices(ctx context.Context, discoveryType string) ([]v1.Service, error) {
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

			// Skip service if it has type annotation and it's not the expected type.
			if v, ok := s.GetAnnotations()[types.DiscoveryTypeLabel]; ok && v != discoveryType {
				continue
			}

			// If the service is marked with the ignore annotation, skip it.
			if v := s.GetAnnotations()[types.DiscoveryAppIgnore]; v == "true" {
				continue
			}

			match, _, err := services.MatchLabels(f.FilterLabels, s.Labels)
			if err != nil {
				return nil, trace.Wrap(err)
			} else if match {
				result = append(result, s)
			} else {
				f.Logger.DebugContext(ctx, "Service doesn't match labels", "service", s.Name)
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
	kubeServices, err := f.getServices(ctx, types.KubernetesMatchersApp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Converting service to apps can involve performing a HTTP ping to the service ports to determine protocol.
	// Both services and ports inside services are processed in parallel to minimize time.
	// We also set limit to prevent potential spike load on a cluster in case there are a lot of services.
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(20)

	// Convert services to resources
	var (
		appsMu sync.Mutex
		apps   types.Apps
	)
	for _, service := range kubeServices {
		service := service
		g.Go(func() error {
			protocolAnnotation := service.GetAnnotations()[types.DiscoveryProtocolLabel]

			ports, err := getServicePorts(service)
			if err != nil {
				f.Logger.ErrorContext(ctx, "could not get ports for the service", "error", err, "service", service.GetName())
				return nil
			}

			portProtocols := map[v1.ServicePort]string{}
			for _, port := range ports {
				switch protocolAnnotation {
				case protoHTTPS, protoHTTP, protoTCP:
					portProtocols[port] = protocolAnnotation
				default:
					if p := autoProtocolDetection(service, port, f.ProtocolChecker); p != protoTCP {
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
					f.Logger.WarnContext(ctx, "Could not get app from a Kubernetes service", "error", err, "service", service.GetName(), "port", port.Port)
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

func (f *KubeAppFetcher) IntegrationName() string {
	// KubeAppFetcher does not have an integration.
	return ""
}

func (f *KubeAppFetcher) GetDiscoveryConfigName() string {
	return f.DiscoveryConfigName
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
//   - If port's name is `http` or number is 80 or 8080, we return `http`
//   - If protocol checker is available it will perform HTTP request to the service fqdn trying to find out protocol. If it
//     gives us result `http` or `https` we return it
func autoProtocolDetection(service v1.Service, port v1.ServicePort, pc ProtocolChecker) string {
	if port.AppProtocol != nil {
		switch p := strings.ToLower(*port.AppProtocol); p {
		case protoHTTP, protoHTTPS:
			return p
		}
	}

	if strings.EqualFold(port.Name, protoHTTPS) || strings.EqualFold(port.TargetPort.StrVal, protoHTTPS) ||
		port.Port == 443 || port.NodePort == 443 || port.TargetPort.IntVal == 443 {

		return protoHTTPS
	}

	if strings.EqualFold(port.Name, protoHTTP) || strings.EqualFold(port.TargetPort.StrVal, protoHTTP) ||
		port.Port == 80 || port.NodePort == 80 || port.TargetPort.IntVal == 80 ||
		port.Port == 8080 || port.NodePort == 8080 || port.TargetPort.IntVal == 8080 {

		return protoHTTP
	}

	if pc != nil {
		if result := pc.CheckProtocol(service, port); result != protoTCP {
			return result
		}
	}

	return protoTCP
}

// ProtocolChecker is an interface used to check what protocol uri serves
type ProtocolChecker interface {
	CheckProtocol(service v1.Service, port v1.ServicePort) string
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
	client *http.Client

	// cacheKubernetesServiceProtocol maps a Kubernetes Service Namespace/Name to a tuple containing the Service's ResourceVersion and the Protocol.
	// When the Kubernetes Service ResourceVersion changes, then we assume the protocol might've changed as well, so the cache is invalidated.
	// Only protocol checkers that require a network connection are cached.
	cacheKubernetesServiceProtocol map[kubernetesNameNamespace]appResourceVersionProtocol
	cacheMU                        sync.RWMutex
}

type appResourceVersionProtocol struct {
	resourceVersion string
	protocol        string
}

type kubernetesNameNamespace struct {
	namespace string
	name      string
}

func NewProtoChecker() *ProtoChecker {
	p := &ProtoChecker{
		client: &http.Client{
			// This is a best-effort scenario, where teleport tries to guess which protocol is being used.
			// Ideally it should either be inferred by the Service's ports or explicitly configured by using annotations on the service.
			Timeout: 500 * time.Millisecond,
		},
		cacheKubernetesServiceProtocol: make(map[kubernetesNameNamespace]appResourceVersionProtocol),
	}

	return p
}

func (p *ProtoChecker) CheckProtocol(service v1.Service, port v1.ServicePort) string {
	if p.client == nil {
		return protoTCP
	}

	key := kubernetesNameNamespace{namespace: service.Namespace, name: service.Name}

	p.cacheMU.RLock()
	versionProtocol, keyIsCached := p.cacheKubernetesServiceProtocol[key]
	p.cacheMU.RUnlock()

	if keyIsCached && versionProtocol.resourceVersion == service.ResourceVersion {
		return versionProtocol.protocol
	}

	var result string

	uri := fmt.Sprintf("https://%s:%d", services.GetServiceFQDN(service), port.Port)
	resp, err := p.client.Head(uri)
	switch {
	case err == nil:
		result = protoHTTPS
		_ = resp.Body.Close()

	case errors.Is(err, http.ErrSchemeMismatch):
		result = protoHTTP

	default:
		result = protoTCP

	}

	p.cacheMU.Lock()
	p.cacheKubernetesServiceProtocol[key] = appResourceVersionProtocol{
		resourceVersion: service.ResourceVersion,
		protocol:        result,
	}
	p.cacheMU.Unlock()

	return result
}
