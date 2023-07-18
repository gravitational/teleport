/*
Copyright 2021 Gravitational, Inc.

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

package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// AppGetter defines interface for fetching application resources.
type AppGetter interface {
	// GetApps returns all application resources.
	GetApps(context.Context) ([]types.Application, error)
	// GetApp returns the specified application resource.
	GetApp(ctx context.Context, name string) (types.Application, error)
}

// Apps defines an interface for managing application resources.
type Apps interface {
	// AppGetter provides methods for fetching application resources.
	AppGetter
	// CreateApp creates a new application resource.
	CreateApp(context.Context, types.Application) error
	// UpdateApp updates an existing application resource.
	UpdateApp(context.Context, types.Application) error
	// DeleteApp removes the specified application resource.
	DeleteApp(ctx context.Context, name string) error
	// DeleteAllApps removes all database resources.
	DeleteAllApps(context.Context) error
}

// MarshalApp marshals Application resource to JSON.
func MarshalApp(app types.Application, opts ...MarshalOption) ([]byte, error) {
	if err := app.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch app := app.(type) {
	case *types.AppV3:
		if !cfg.PreserveResourceID {
			copy := *app
			copy.SetResourceID(0)
			app = &copy
		}
		return utils.FastMarshal(app)
	default:
		return nil, trace.BadParameter("unsupported app resource %T", app)
	}
}

// UnmarshalApp unmarshals Application resource from JSON.
func UnmarshalApp(data []byte, opts ...MarshalOption) (types.Application, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing app resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var app types.AppV3
		if err := utils.FastUnmarshal(data, &app); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := app.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			app.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			app.SetExpiry(cfg.Expires)
		}
		return &app, nil
	}
	return nil, trace.BadParameter("unsupported app resource version %q", h.Version)
}

// MarshalAppServer marshals the AppServer resource to JSON.
func MarshalAppServer(appServer types.AppServer, opts ...MarshalOption) ([]byte, error) {
	if err := appServer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch appServer := appServer.(type) {
	case *types.AppServerV3:
		if !cfg.PreserveResourceID {
			copy := *appServer
			copy.SetResourceID(0)
			appServer = &copy
		}
		return utils.FastMarshal(appServer)
	default:
		return nil, trace.BadParameter("unsupported app server resource %T", appServer)
	}
}

// UnmarshalAppServer unmarshals AppServer resource from JSON.
func UnmarshalAppServer(data []byte, opts ...MarshalOption) (types.AppServer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing app server data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var s types.AppServerV3
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported app server resource version %q", h.Version)
}

// NewApplicationsFromKubeService creates application resources from kubernetes service. One service result
// in multiple application resources if there are multiple ports exposed.
func NewApplicationsFromKubeService(service v1.Service, clusterName string, pi ProtocolChecker) ([]types.Application, error) {
	var apps types.Apps
	protocol := service.GetAnnotations()[types.DiscoveryProtocolLabel]

	ports, err := getServicePorts(service)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var appsMu sync.Mutex
	g := errgroup.Group{}
	g.SetLimit(5)
	for _, port := range ports {
		port := port
		g.Go(func() error {
			appURI := getAppURI(getServiceFQDN(service), port, protocol, pi)

			// By default we are looking for http apps, we only create tcp apps if protocol annotation
			// explicitly specifies it.
			if strings.HasPrefix(appURI, protoTCP) && protocol != protoTCP {
				return nil
			}

			rewriteConfig, err := getAppRewriteConfig(service.GetAnnotations())
			if err != nil {
				return trace.Wrap(err)
			}
			a, err := types.NewAppV3(types.Metadata{
				Name:        getAppName(service.GetName(), service.GetNamespace(), clusterName, ""),
				Description: fmt.Sprintf("Discovered application in Kubernetes cluster %q", clusterName),
				Labels:      getAppLabels(service.GetLabels(), clusterName),
			}, types.AppSpecV3{
				URI: appURI,
				// Temporary usage to have app's name with port name, in case there are more than one app
				// created from this service.
				PublicAddr: getAppName(service.GetName(), service.GetNamespace(), clusterName, port.Name),
				Rewrite:    rewriteConfig,
			})
			if err != nil {
				return trace.Wrap(err)
			}
			appsMu.Lock()
			apps = append(apps, a)
			appsMu.Unlock()
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If we ended up with more than one app from the service, we'll have port name in the app name to distinguish them.
	// We can only do this now because we don't know how many apps there will be until all ports are processed by errgroup.
	for i := range apps {
		if len(apps) > 1 {
			apps[i].SetName(apps[i].(*types.AppV3).Spec.PublicAddr)
		}
		apps[i].(*types.AppV3).Spec.PublicAddr = "" // Clear temporary used for name field.
	}

	return apps, nil
}

// ProtocolChecker is an interface used to check what protocol uri serves
type ProtocolChecker interface {
	CheckProtocol(uri string) string
}

func getServiceFQDN(s v1.Service) string {
	host := s.GetName()
	if s.Spec.Type == v1.ServiceTypeExternalName {
		host = s.Spec.ExternalName
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local", host, s.GetNamespace())
}

const (
	protoHTTPS = "https"
	protoHTTP  = "http"
	protoTCP   = "tcp"
)

func getAppURI(serviceFQDN string, port v1.ServicePort, protocolAnnotation string, pc ProtocolChecker) string {
	constructURI := func(p string) string { return fmt.Sprintf("%s://%s:%d", p, serviceFQDN, port.Port) }

	proto := strings.ToLower(protocolAnnotation)
	if proto == protoHTTP || proto == protoHTTPS {
		return constructURI(proto)
	}

	if port.AppProtocol != nil {
		switch strings.ToLower(*port.AppProtocol) {
		case protoHTTP:
			return constructURI(protoHTTP)
		case protoHTTPS:
			return constructURI(protoHTTPS)
		}
	}

	if port.Port == 443 || strings.ToLower(port.Name) == protoHTTPS {
		return constructURI(protoHTTPS)
	}

	if pc != nil {
		result := pc.CheckProtocol(fmt.Sprintf("%s:%d", serviceFQDN, port.Port))
		if result != protoTCP {
			return constructURI(result)
		}
	}

	if port.Port == 80 || port.Port == 8080 || strings.ToLower(port.Name) == protoHTTP {
		return constructURI(protoHTTP)
	}

	return constructURI(protoTCP)
}

func getAppRewriteConfig(annotations map[string]string) (*types.Rewrite, error) {
	rewrite := annotations[types.DiscoveryAppRewriteLabel]
	if rewrite == "" {
		return nil, nil
	}

	rw := types.Rewrite{}
	reader := strings.NewReader(rewrite)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	err := decoder.Decode(&rw)
	if err != nil {
		return nil, trace.Wrap(err, "failed decoding rewrite config")
	}

	return &rw, nil
}

func getAppName(serviceName, namespace, clusterName, portName string) string {
	clusterName = strings.ReplaceAll(clusterName, ".", "-")
	if portName != "" {
		return fmt.Sprintf("%s-%s-%s-%s", serviceName, portName, namespace, clusterName)
	}
	return fmt.Sprintf("%s-%s-%s", serviceName, namespace, clusterName)
}

func getAppLabels(serviceLabels map[string]string, clusterName string) map[string]string {
	result := make(map[string]string, len(serviceLabels)+1)

	for k, v := range serviceLabels {
		if !types.IsValidLabelKey(k) {
			logrus.Debugf("Skipping label %q as invalid while creating app labels from service", k)
			continue
		}

		result[k] = v
	}
	result[types.KubernetesClusterLabel] = clusterName

	return result
}

func getServicePorts(s v1.Service) ([]v1.ServicePort, error) {
	preferredPort := ""
	for k, v := range s.GetAnnotations() {
		if k == types.DiscoveryPortLabel {
			preferredPort = v
		}
	}
	availablePorts := []v1.ServicePort{}
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
		return nil, trace.BadParameter("Specified preferred port %s is absent among available service ports", preferredPort)
	}

	return availablePorts, nil
}

type protoChecker struct {
	InsecureSkipVerify bool
}

func (p *protoChecker) CheckProtocol(uri string) string {
	client := http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: p.InsecureSkipVerify,
			},
		},
	}

	resp, err := client.Head(fmt.Sprintf("https://%s", uri))
	if err == nil {
		_ = resp.Body.Close()
		return protoHTTPS
	} else if strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") {
		return protoHTTP
	}

	return protoTCP
}
