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

package types

import (
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// ReverseTunnel is SSH reverse tunnel established between a local Proxy
// and a remote Proxy. It helps to bypass firewall restrictions, so local
// clusters don't need to have the cluster involved
type ReverseTunnel interface {
	// Resource provides common methods for resource objects
	Resource
	// GetClusterName returns name of the cluster
	GetClusterName() string
	// SetClusterName sets cluster name
	SetClusterName(name string)
	// GetType gets the type of ReverseTunnel.
	GetType() TunnelType
	// SetType sets the type of ReverseTunnel.
	SetType(TunnelType)
	// GetDialAddrs returns list of dial addresses for this cluster
	GetDialAddrs() []string
}

// NewReverseTunnel returns new version of reverse tunnel
func NewReverseTunnel(clusterName string, dialAddrs []string) (ReverseTunnel, error) {
	r := &ReverseTunnelV2{
		Metadata: Metadata{
			Name: clusterName,
		},
		Spec: ReverseTunnelSpecV2{
			ClusterName: clusterName,
			DialAddrs:   dialAddrs,
		},
	}
	if err := r.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}

// GetVersion returns resource version
func (r *ReverseTunnelV2) GetVersion() string {
	return r.Version
}

// GetKind returns resource kind
func (r *ReverseTunnelV2) GetKind() string {
	return r.Kind
}

// GetSubKind returns resource sub kind
func (r *ReverseTunnelV2) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *ReverseTunnelV2) SetSubKind(s string) {
	r.SubKind = s
}

// GetRevision returns the revision
func (r *ReverseTunnelV2) GetRevision() string {
	return r.Metadata.GetRevision()
}

// SetRevision sets the revision
func (r *ReverseTunnelV2) SetRevision(rev string) {
	r.Metadata.SetRevision(rev)
}

// GetMetadata returns object metadata
func (r *ReverseTunnelV2) GetMetadata() Metadata {
	return r.Metadata
}

// SetExpiry sets expiry time for the object
func (r *ReverseTunnelV2) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (r *ReverseTunnelV2) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// GetName returns the name of the User
func (r *ReverseTunnelV2) GetName() string {
	return r.Metadata.Name
}

// SetName sets the name of the User
func (r *ReverseTunnelV2) SetName(e string) {
	r.Metadata.Name = e
}

// setStaticFields sets static resource header and metadata fields.
func (r *ReverseTunnelV2) setStaticFields() {
	r.Kind = KindReverseTunnel
	r.Version = V2
}

// CheckAndSetDefaults checks and sets defaults
func (r *ReverseTunnelV2) CheckAndSetDefaults() error {
	r.setStaticFields()
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if strings.TrimSpace(r.Spec.ClusterName) == "" {
		return trace.BadParameter("reverse tunnel validation error: empty cluster name")
	}
	if len(r.Spec.DialAddrs) == 0 {
		return trace.BadParameter("invalid dial address for reverse tunnel '%v'", r.Spec.ClusterName)
	}

	return nil
}

// SetClusterName sets name of a cluster
func (r *ReverseTunnelV2) SetClusterName(name string) {
	r.Spec.ClusterName = name
}

// GetClusterName returns name of the cluster
func (r *ReverseTunnelV2) GetClusterName() string {
	return r.Spec.ClusterName
}

// GetType gets the type of ReverseTunnel.
func (r *ReverseTunnelV2) GetType() TunnelType {
	if string(r.Spec.Type) == "" {
		return ProxyTunnel
	}
	return r.Spec.Type
}

// SetType sets the type of ReverseTunnel.
func (r *ReverseTunnelV2) SetType(tt TunnelType) {
	r.Spec.Type = tt
}

// GetDialAddrs returns list of dial addresses for this cluster
func (r *ReverseTunnelV2) GetDialAddrs() []string {
	return r.Spec.DialAddrs
}
