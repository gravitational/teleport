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
	"time"

	"github.com/gravitational/teleport/api/defaults"

	"github.com/gravitational/trace"
)

// StaticTokens define a list of static []ProvisionToken used to provision a
// node. StaticTokens is a configuration resource, never create more than one instance
// of it.
type StaticTokens interface {
	// Resource provides common resource properties.
	Resource
	// SetStaticTokens sets the list of static tokens used to provision nodes.
	SetStaticTokens([]ProvisionToken)
	// GetStaticTokens gets the list of static tokens used to provision nodes.
	GetStaticTokens() []ProvisionToken
	// CheckAndSetDefaults checks and set default values for missing fields.
	CheckAndSetDefaults() error
}

// NewStaticTokens is a convenience wrapper to create a StaticTokens resource.
func NewStaticTokens(spec StaticTokensSpecV2) (StaticTokens, error) {
	st := StaticTokensV2{
		Kind:    KindStaticTokens,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameStaticTokens,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
	if err := st.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &st, nil
}

// GetVersion returns resource version
func (c *StaticTokensV2) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *StaticTokensV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *StaticTokensV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *StaticTokensV2) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetResourceID returns resource ID
func (c *StaticTokensV2) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *StaticTokensV2) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetName returns the name of the StaticTokens resource.
func (c *StaticTokensV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the StaticTokens resource.
func (c *StaticTokensV2) SetName(e string) {
	c.Metadata.Name = e
}

// Expiry returns object expiry setting
func (c *StaticTokensV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *StaticTokensV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (c *StaticTokensV2) SetTTL(clock Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *StaticTokensV2) GetMetadata() Metadata {
	return c.Metadata
}

// SetStaticTokens sets the list of static tokens used to provision nodes.
func (c *StaticTokensV2) SetStaticTokens(s []ProvisionToken) {
	c.Spec.StaticTokens = ProvisionTokensToV1(s)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (c *StaticTokensV2) GetStaticTokens() []ProvisionToken {
	return ProvisionTokensFromV1(c.Spec.StaticTokens)
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *StaticTokensV2) CheckAndSetDefaults() error {
	// make sure we have defaults for all metadata fields
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// String represents a human readable version of static provisioning tokens.
func (c *StaticTokensV2) String() string {
	return fmt.Sprintf("StaticTokens(%v)", c.Spec.StaticTokens)
}
