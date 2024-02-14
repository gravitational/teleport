/*
Copyright 2024 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// AccessMonitoringRuleSubkind represents the type of the AccessMonitoringRule, e.g., access request, MDM etc.
type AccessMonitoringRuleSubkind string

const (
	// AccessMonitoringRuleSubkindUnknown is returned when no AccessMonitoringRule subkind matches.
	AccessMonitoringRuleSubkindUnknown AccessMonitoringRuleSubkind = ""
	DefaultAccessMonitoringRuleExpiry time.Duration= time.Hour*24*7
)

// AccessMonitoringRule represents a AccessMonitoringRule instance
type AccessMonitoringRule interface {
	// Resource provides common resource methods.
	Resource
	Clone() AccessMonitoringRule
}

// NewAccessMonitoringRuleV1 creates a new AccessMonitoringRuleV1 resource.
func NewAccessMonitoringRuleV1(metadata Metadata, spec AccessMonitoringRuleSpec) *AccessMonitoringRuleV1 {
	p := &AccessMonitoringRuleV1{
		Metadata: metadata,
		Spec:     spec,
	}
	return p
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (p *AccessMonitoringRuleV1) CheckAndSetDefaults() error {
	if err := p.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	// TODO: Decide on better expiry
	if p.GetMetadata().Expires == nil {
		expiry := time.Now().Add(DefaultAccessMonitoringRuleExpiry).UTC()
		p.Metadata.Expires = &expiry
	}
	return nil
}

// Clone returns a copy of the AccessMonitoringRule instance
func (p *AccessMonitoringRuleV1) Clone() AccessMonitoringRule {
	return utils.CloneProtoMsg(p)
}

// GetVersion returns resource version
func (p *AccessMonitoringRuleV1) GetVersion() string {
	return p.Version
}

// GetKind returns resource kind
func (p *AccessMonitoringRuleV1) GetKind() string {
	return p.Kind
}

// GetSubKind returns resource sub kind
func (p *AccessMonitoringRuleV1) GetSubKind() string {
	return p.SubKind
}

// SetSubKind sets resource subkind
func (p *AccessMonitoringRuleV1) SetSubKind(s string) {
	p.SubKind = s
}

// GetResourceID returns resource ID
func (p *AccessMonitoringRuleV1) GetResourceID() int64 {
	return p.Metadata.ID
}

// SetResourceID sets resource ID
func (p *AccessMonitoringRuleV1) SetResourceID(id int64) {
	p.Metadata.ID = id
}

// GetRevision returns the revision
func (p *AccessMonitoringRuleV1) GetRevision() string {
	return p.Metadata.GetRevision()
}

// SetRevision sets the revision
func (p *AccessMonitoringRuleV1) SetRevision(rev string) {
	p.Metadata.SetRevision(rev)
}

// GetMetadata returns object metadata
func (p *AccessMonitoringRuleV1) GetMetadata() Metadata {
	return p.Metadata
}

// SetMetadata sets object metadata
func (p *AccessMonitoringRuleV1) SetMetadata(meta Metadata) {
	p.Metadata = meta
}

// Expiry returns expiry time for the object
func (p *AccessMonitoringRuleV1) Expiry() time.Time {
	return p.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (p *AccessMonitoringRuleV1) SetExpiry(expires time.Time) {
	p.Metadata.SetExpiry(expires)
}

// GetName returns the name of the User
func (p *AccessMonitoringRuleV1) GetName() string {
	return p.Metadata.Name
}

// SetName sets the name of the User
func (p *AccessMonitoringRuleV1) SetName(e string) {
	p.Metadata.Name = e
}
