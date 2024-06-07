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
}

// NewStaticTokens is a convenience wrapper to create a StaticTokens resource.
func NewStaticTokens(spec StaticTokensSpecV2) (StaticTokens, error) {
	st := &StaticTokensV2{Spec: spec}
	if err := st.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return st, nil
}

// DefaultStaticTokens is used to get the default static tokens (empty list)
// when nothing is specified in file configuration.
func DefaultStaticTokens() StaticTokens {
	token, _ := NewStaticTokens(StaticTokensSpecV2{})
	return token
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

// GetRevision returns the revision
func (c *StaticTokensV2) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *StaticTokensV2) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
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
	return ProvisionTokensFromStatic(c.Spec.StaticTokens)
}

// setStaticFields sets static resource header and metadata fields.
func (c *StaticTokensV2) setStaticFields() {
	c.Kind = KindStaticTokens
	c.Version = V2
	c.Metadata.Name = MetaNameStaticTokens
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *StaticTokensV2) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// String represents a human readable version of static provisioning tokens.
func (c *StaticTokensV2) String() string {
	return fmt.Sprintf("StaticTokens(%v)", c.Spec.StaticTokens)
}
