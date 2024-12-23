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
	"fmt"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// TrustedCluster holds information needed for a cluster that can not be directly
// accessed (maybe be behind firewall without any open ports) to join a parent cluster.
type TrustedCluster interface {
	// ResourceWithOrigin provides common resource properties
	ResourceWithOrigin
	// SetMetadata sets object metadata
	SetMetadata(meta Metadata)
	// GetEnabled returns the state of the TrustedCluster.
	GetEnabled() bool
	// SetEnabled enables (handshake and add ca+reverse tunnel) or disables TrustedCluster.
	SetEnabled(bool)
	// CombinedMapping is used to specify combined mapping from legacy property Roles
	// and new property RoleMap
	CombinedMapping() RoleMap
	// GetRoleMap returns role map property
	GetRoleMap() RoleMap
	// SetRoleMap sets role map
	SetRoleMap(m RoleMap)
	// GetRoles returns the roles for the certificate authority.
	GetRoles() []string
	// SetRoles sets the roles for the certificate authority.
	SetRoles([]string)
	// GetToken returns the authorization and authentication token.
	GetToken() string
	// SetToken sets the authorization and authentication.
	SetToken(string)
	// GetProxyAddress returns the address of the proxy server.
	GetProxyAddress() string
	// SetProxyAddress sets the address of the proxy server.
	SetProxyAddress(string)
	// GetReverseTunnelAddress returns the address of the reverse tunnel.
	GetReverseTunnelAddress() string
	// SetReverseTunnelAddress sets the address of the reverse tunnel.
	SetReverseTunnelAddress(string)
	// CanChangeStateTo checks the TrustedCluster can transform into another.
	CanChangeStateTo(TrustedCluster) error
	// Clone returns a deep copy of the TrustedCluster.
	Clone() TrustedCluster
}

// NewTrustedCluster is a convenience way to create a TrustedCluster resource.
func NewTrustedCluster(name string, spec TrustedClusterSpecV2) (TrustedCluster, error) {
	c := &TrustedClusterV2{
		Metadata: Metadata{
			Name: name,
		},
		Spec: spec,
	}
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// setStaticFields sets static resource header and metadata fields.
func (c *TrustedClusterV2) setStaticFields() {
	c.Kind = KindTrustedCluster
	c.Version = V2
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (c *TrustedClusterV2) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// This is to force users to migrate
	if len(c.Spec.Roles) != 0 && len(c.Spec.RoleMap) != 0 {
		return trace.BadParameter("should set either 'roles' or 'role_map', not both")
	}
	// Imply that by default proxy listens on the same port for
	// web and reverse tunnel connections
	if c.Spec.ReverseTunnelAddress == "" {
		c.Spec.ReverseTunnelAddress = c.Spec.ProxyAddress
	}
	return nil
}

// GetVersion returns resource version
func (c *TrustedClusterV2) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *TrustedClusterV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *TrustedClusterV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *TrustedClusterV2) SetSubKind(s string) {
	c.SubKind = s
}

// GetRevision returns the revision
func (c *TrustedClusterV2) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *TrustedClusterV2) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
}

// CombinedMapping is used to specify combined mapping from legacy property Roles
// and new property RoleMap
func (c *TrustedClusterV2) CombinedMapping() RoleMap {
	if len(c.Spec.Roles) != 0 {
		return []RoleMapping{{Remote: Wildcard, Local: c.Spec.Roles}}
	}
	return c.Spec.RoleMap
}

// GetRoleMap returns role map property
func (c *TrustedClusterV2) GetRoleMap() RoleMap {
	return c.Spec.RoleMap
}

// SetRoleMap sets role map
func (c *TrustedClusterV2) SetRoleMap(m RoleMap) {
	c.Spec.RoleMap = m
}

// GetMetadata returns object metadata
func (c *TrustedClusterV2) GetMetadata() Metadata {
	return c.Metadata
}

// SetMetadata sets object metadata
func (c *TrustedClusterV2) SetMetadata(meta Metadata) {
	c.Metadata = meta
}

// SetExpiry sets expiry time for the object
func (c *TrustedClusterV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (c *TrustedClusterV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetName returns the name of the TrustedCluster.
func (c *TrustedClusterV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the TrustedCluster.
func (c *TrustedClusterV2) SetName(e string) {
	c.Metadata.Name = e
}

// Origin returns the origin value of the resource.
func (c *TrustedClusterV2) Origin() string {
	return c.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (c *TrustedClusterV2) SetOrigin(origin string) {
	c.Metadata.SetOrigin(origin)
}

// GetEnabled returns the state of the TrustedCluster.
func (c *TrustedClusterV2) GetEnabled() bool {
	return c.Spec.Enabled
}

// SetEnabled enables (handshake and add ca+reverse tunnel) or disables TrustedCluster.
func (c *TrustedClusterV2) SetEnabled(e bool) {
	c.Spec.Enabled = e
}

// GetRoles returns the roles for the certificate authority.
func (c *TrustedClusterV2) GetRoles() []string {
	return c.Spec.Roles
}

// SetRoles sets the roles for the certificate authority.
func (c *TrustedClusterV2) SetRoles(e []string) {
	c.Spec.Roles = e
}

// GetToken returns the authorization and authentication token.
func (c *TrustedClusterV2) GetToken() string {
	return c.Spec.Token
}

// SetToken sets the authorization and authentication.
func (c *TrustedClusterV2) SetToken(e string) {
	c.Spec.Token = e
}

// GetProxyAddress returns the address of the proxy server.
func (c *TrustedClusterV2) GetProxyAddress() string {
	return c.Spec.ProxyAddress
}

// SetProxyAddress sets the address of the proxy server.
func (c *TrustedClusterV2) SetProxyAddress(e string) {
	c.Spec.ProxyAddress = e
}

// GetReverseTunnelAddress returns the address of the reverse tunnel.
func (c *TrustedClusterV2) GetReverseTunnelAddress() string {
	return c.Spec.ReverseTunnelAddress
}

// SetReverseTunnelAddress sets the address of the reverse tunnel.
func (c *TrustedClusterV2) SetReverseTunnelAddress(e string) {
	c.Spec.ReverseTunnelAddress = e
}

// CanChangeStateTo checks if the state change is allowed or not. If not, returns
// an error explaining the reason.
func (c *TrustedClusterV2) CanChangeStateTo(t TrustedCluster) error {
	immutableFieldErr := func(name string) error {
		return trace.BadParameter("can not update %s for existing leaf cluster, delete and re-create this leaf cluster with updated %s", name, name)
	}
	if c.GetToken() != t.GetToken() {
		return immutableFieldErr("token")
	}
	if c.GetProxyAddress() != t.GetProxyAddress() {
		return immutableFieldErr("web_proxy_address")
	}
	if c.GetReverseTunnelAddress() != t.GetReverseTunnelAddress() {
		return immutableFieldErr("tunnel_addr")
	}
	if !slices.Equal(c.GetRoles(), t.GetRoles()) {
		return immutableFieldErr("roles")
	}
	return nil
}

func (c *TrustedClusterV2) Clone() TrustedCluster {
	return utils.CloneProtoMsg(c)
}

// String represents a human readable version of trusted cluster settings.
func (c *TrustedClusterV2) String() string {
	return fmt.Sprintf("TrustedCluster(Enabled=%v,Roles=%v,Token=%v,ProxyAddress=%v,ReverseTunnelAddress=%v)",
		c.Spec.Enabled, c.Spec.Roles, c.Spec.Token, c.Spec.ProxyAddress, c.Spec.ReverseTunnelAddress)
}

// RoleMap is a list of mappings
type RoleMap []RoleMapping

// IsEqual validates that two roles maps are equivalent.
func (r RoleMap) IsEqual(other RoleMap) bool {
	return slices.EqualFunc(r, other, func(a RoleMapping, b RoleMapping) bool {
		return a.Remote == b.Remote && slices.Equal(a.Local, b.Local)
	})
}

// SortedTrustedCluster sorts clusters by name
type SortedTrustedCluster []TrustedCluster

// Len returns the length of a list.
func (s SortedTrustedCluster) Len() int {
	return len(s)
}

// Less compares items by name.
func (s SortedTrustedCluster) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

// Swap swaps two items in a list.
func (s SortedTrustedCluster) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
