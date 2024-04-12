/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"fmt"
	"sort"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/types/compare"
	"github.com/gravitational/teleport/api/utils"
)

var _ compare.IsEqual[KubeServer] = (*KubernetesServerV3)(nil)

// KubeServer represents a single Kubernetes server.
type KubeServer interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns server namespace.
	GetNamespace() string
	// GetTeleportVersion returns the teleport version the server is running on.
	GetTeleportVersion() string
	// GetHostname returns the server hostname.
	GetHostname() string
	// GetHostID returns ID of the host the server is running on.
	GetHostID() string
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() Rotation
	// SetRotation sets the state of certificate authority rotation.
	SetRotation(Rotation)
	// String returns string representation of the server.
	String() string
	// Copy returns a copy of this kube server object.
	Copy() KubeServer
	// CloneResource returns a copy of the KubeServer as a ResourceWithLabels
	CloneResource() ResourceWithLabels
	// GetCluster returns the Kubernetes Cluster this kube server proxies.
	GetCluster() KubeCluster
	// SetCluster sets the kube cluster this kube server server proxies.
	SetCluster(KubeCluster) error
	// ProxiedService provides common methods for a proxied service.
	ProxiedService
}

// NewKubernetesServerV3 creates a new kube server instance.
func NewKubernetesServerV3(meta Metadata, spec KubernetesServerSpecV3) (*KubernetesServerV3, error) {
	s := &KubernetesServerV3{
		Metadata: meta,
		Spec:     spec,
	}
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
}

// NewKubernetesServerV3FromCluster creates a new kubernetes server from the provided clusters.
func NewKubernetesServerV3FromCluster(cluster *KubernetesClusterV3, hostname, hostID string) (*KubernetesServerV3, error) {
	return NewKubernetesServerV3(Metadata{
		Name: cluster.GetName(),
	}, KubernetesServerSpecV3{
		Hostname: hostname,
		HostID:   hostID,
		Cluster:  cluster,
	})
}

// GetVersion returns the kubernetes server resource version.
func (s *KubernetesServerV3) GetVersion() string {
	return s.Version
}

// GetTeleportVersion returns the Teleport version the server is running.
func (s *KubernetesServerV3) GetTeleportVersion() string {
	return s.Spec.Version
}

// GetHostname returns the kubernetes server hostname.
func (s *KubernetesServerV3) GetHostname() string {
	return s.Spec.Hostname
}

// GetHostID returns ID of the host the server is running on.
func (s *KubernetesServerV3) GetHostID() string {
	return s.Spec.HostID
}

// GetKind returns the resource kind.
func (s *KubernetesServerV3) GetKind() string {
	return s.Kind
}

// GetSubKind returns the resource subkind.
func (s *KubernetesServerV3) GetSubKind() string {
	return s.SubKind
}

// SetSubKind sets the resource subkind.
func (s *KubernetesServerV3) SetSubKind(sk string) {
	s.SubKind = sk
}

// GetResourceID returns the resource ID.
func (s *KubernetesServerV3) GetResourceID() int64 {
	return s.Metadata.ID
}

// SetResourceID sets the resource ID.
func (s *KubernetesServerV3) SetResourceID(id int64) {
	s.Metadata.ID = id
}

// GetRevision returns the revision
func (s *KubernetesServerV3) GetRevision() string {
	return s.Metadata.GetRevision()
}

// SetRevision sets the revision
func (s *KubernetesServerV3) SetRevision(rev string) {
	s.Metadata.SetRevision(rev)
}

// GetMetadata returns the resource metadata.
func (s *KubernetesServerV3) GetMetadata() Metadata {
	return s.Metadata
}

// GetNamespace returns the resource namespace.
func (s *KubernetesServerV3) GetNamespace() string {
	return s.Metadata.Namespace
}

// SetExpiry sets the resource expiry time.
func (s *KubernetesServerV3) SetExpiry(expiry time.Time) {
	s.Metadata.SetExpiry(expiry)
}

// Expiry returns the resource expiry time.
func (s *KubernetesServerV3) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// GetName returns the resource name.
func (s *KubernetesServerV3) GetName() string {
	return s.Metadata.Name
}

// SetName sets the resource name.
func (s *KubernetesServerV3) SetName(name string) {
	s.Metadata.Name = name
}

// GetRotation returns the server CA rotation state.
func (s *KubernetesServerV3) GetRotation() Rotation {
	return s.Spec.Rotation
}

// SetRotation sets the server CA rotation state.
func (s *KubernetesServerV3) SetRotation(r Rotation) {
	s.Spec.Rotation = r
}

// GetCluster returns the cluster this kube server proxies.
func (s *KubernetesServerV3) GetCluster() KubeCluster {
	if s.Spec.Cluster == nil {
		return nil
	}
	return s.Spec.Cluster
}

// SetCluster sets the cluster this kube server proxies.
func (s *KubernetesServerV3) SetCluster(cluster KubeCluster) error {
	clusterV3, ok := cluster.(*KubernetesClusterV3)
	if !ok {
		return trace.BadParameter("expected *KubernetesClusterV3, got %T", cluster)
	}
	s.Spec.Cluster = clusterV3
	return nil
}

// String returns the server string representation.
func (s *KubernetesServerV3) String() string {
	return fmt.Sprintf("KubeServer(Name=%v, Version=%v, Hostname=%v, HostID=%v, Cluster=%v)",
		s.GetName(), s.GetTeleportVersion(), s.GetHostname(), s.GetHostID(), s.GetCluster())
}

// setStaticFields sets static resource header and metadata fields.
func (s *KubernetesServerV3) setStaticFields() {
	s.Kind = KindKubeServer
	s.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (s *KubernetesServerV3) CheckAndSetDefaults() error {
	s.setStaticFields()
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.Spec.HostID == "" {
		return trace.BadParameter("missing kube server HostID")
	}
	if s.Spec.Version == "" {
		s.Spec.Version = api.Version
	}
	if s.Spec.Cluster == nil {
		return trace.BadParameter("missing kube server Cluster")
	}

	if err := s.Spec.Cluster.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Origin returns the origin value of the resource.
func (s *KubernetesServerV3) Origin() string {
	return s.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (s *KubernetesServerV3) SetOrigin(origin string) {
	s.Metadata.SetOrigin(origin)
}

// GetProxyIDs returns a list of proxy ids this server is connected to.
func (s *KubernetesServerV3) GetProxyIDs() []string {
	return s.Spec.ProxyIDs
}

// SetProxyID sets the proxy ids this server is connected to.
func (s *KubernetesServerV3) SetProxyIDs(proxyIDs []string) {
	s.Spec.ProxyIDs = proxyIDs
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (s *KubernetesServerV3) GetLabel(key string) (value string, ok bool) {
	if s.Spec.Cluster != nil {
		if v, ok := s.Spec.Cluster.GetLabel(key); ok {
			return v, ok
		}
	}

	v, ok := s.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns all resource's labels. Considering:
// * Static labels from `Metadata.Labels` and `Spec.Cluster`.
// * Dynamic labels from `Spec.Cluster.Spec`.
func (s *KubernetesServerV3) GetAllLabels() map[string]string {
	staticLabels := make(map[string]string)
	for name, value := range s.Metadata.Labels {
		staticLabels[name] = value
	}

	var dynamicLabels map[string]CommandLabelV2
	if s.Spec.Cluster != nil {
		for name, value := range s.Spec.Cluster.Metadata.Labels {
			staticLabels[name] = value
		}

		dynamicLabels = s.Spec.Cluster.Spec.DynamicLabels
	}

	return CombineLabels(staticLabels, dynamicLabels)
}

// GetStaticLabels returns the kube server static labels.
func (s *KubernetesServerV3) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the kube server static labels.
func (s *KubernetesServerV3) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// Copy returns a copy of this kube server object.
func (s *KubernetesServerV3) Copy() KubeServer {
	return utils.CloneProtoMsg(s)
}

// CloneResource returns a copy of this kube server object.
func (s *KubernetesServerV3) CloneResource() ResourceWithLabels {
	return s.Copy()
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *KubernetesServerV3) MatchSearch(values []string) bool {
	return MatchSearch(nil, values, nil)
}

// IsEqual determines if two kube server resources are equivalent to one another.
func (k *KubernetesServerV3) IsEqual(i KubeServer) bool {
	if other, ok := i.(*KubernetesServerV3); ok {
		return deriveTeleportEqualKubernetesServerV3(k, other)
	}
	return false
}

// KubeServers represents a list of kube servers.
type KubeServers []KubeServer

// Len returns the slice length.
func (s KubeServers) Len() int { return len(s) }

// Less compares kube servers by name and host ID.
func (s KubeServers) Less(i, j int) bool {
	switch {
	case s[i].GetName() < s[j].GetName():
		return true
	case s[i].GetName() > s[j].GetName():
		return false
	default:
		return s[i].GetHostID() < s[j].GetHostID()
	}
}

// Swap swaps two kube servers.
func (s KubeServers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// ToMap returns these kubernetes clusters as a map keyed by cluster name.
func (s KubeServers) ToMap() map[string]KubeServer {
	m := make(map[string]KubeServer, len(s))
	for _, kubeServer := range s {
		m[kubeServer.GetName()] = kubeServer
	}
	return m
}

// SortByCustom custom sorts by given sort criteria.
func (s KubeServers) SortByCustom(sortBy SortBy) error {
	if sortBy.Field == "" {
		return nil
	}

	// We assume sorting by type KubeServer, we are really
	// wanting to sort its contained resource Cluster.
	isDesc := sortBy.IsDesc
	switch sortBy.Field {
	case ResourceMetadataName:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetCluster().GetName(), s[j].GetCluster().GetName(), isDesc)
		})
	case ResourceSpecDescription:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetCluster().GetDescription(), s[j].GetCluster().GetDescription(), isDesc)
		})
	default:
		return trace.NotImplemented("sorting by field %q for resource %q is not supported", sortBy.Field, KindKubeServer)
	}

	return nil
}

// AsResources returns kube servers as type resources with labels.
func (s KubeServers) AsResources() []ResourceWithLabels {
	resources := make([]ResourceWithLabels, len(s))
	for i, server := range s {
		resources[i] = ResourceWithLabels(server)
	}
	return resources
}

// GetFieldVals returns list of select field values.
func (s KubeServers) GetFieldVals(field string) ([]string, error) {
	vals := make([]string, 0, len(s))
	switch field {
	case ResourceMetadataName:
		for _, server := range s {
			vals = append(vals, server.GetCluster().GetName())
		}
	case ResourceSpecDescription:
		for _, server := range s {
			vals = append(vals, server.GetCluster().GetDescription())
		}
	default:
		return nil, trace.NotImplemented("getting field %q for resource %q is not supported", field, KindKubeServer)
	}

	return vals, nil
}
