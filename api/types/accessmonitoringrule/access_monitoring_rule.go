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

package accessmonitoringrule

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
)

// AccessMonitoringRuleSubkind represents the type of the AccessMonitoringRule, e.g., access request, MDM etc.
type AccessMonitoringRuleSubkind string

const (
	// AccessMonitoringRuleSubkindUnknown is returned when no AccessMonitoringRule subkind matches.
	AccessMonitoringRuleSubkindUnknown AccessMonitoringRuleSubkind = ""
	DefaultAccessMonitoringRuleExpiry  time.Duration               = time.Hour * 24 * 7
)

// AccessMonitoringRule represents a AccessMonitoringRule instance
type AccessMonitoringRule struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the access monitoring rule.
	Spec Spec `json:"spec" yaml:"spec"`
}

// Spec is the specification for an access monitoring rule.
type Spec struct {
	// Subjects the rule operates on, can be a resource kind or a particular resource property.
	Subjects []string `json:"subjects" yaml:"subjects"`
	// States are the desired state which the monitoring rule is attempting to bring the subjects matching the condition to.
	States []string `json:"states" yaml:"states"`
	// Condition is a predicate expression that operates on the specified subject resources,
	// and determines whether the subject will be moved into desired state.
	Condition string `json:"condition" yaml:"condition"`
	// Notification defines the notification configuration used if rule is triggered.
	Notification Notification `json:"notification" yaml:"notification"`
}

// Notification defines the notification configuration used if an access monitoring rule is triggered.
type Notification struct {
	// Name is the name of the plugin to which this configuration should apply.
	Name string `json:"name" yaml:"name"`
	// Recipients is the list of recipients the plugin should notify.
	Recipients []string `json:"recipients" yaml:"recipients"`
}

// NewAccessMonitoringRule creates a new AccessMonitoringRule resource.
func NewAccessMonitoringRule(metadata header.Metadata, spec Spec) *AccessMonitoringRule {
	return &AccessMonitoringRule{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (p *AccessMonitoringRule) CheckAndSetDefaults() error {
	if err := p.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if p.GetMetadata().Expires == nil {
		expiry := time.Now().Add(DefaultAccessMonitoringRuleExpiry).UTC()
		p.Metadata.Expires = expiry
	}
	return nil
}

// GetVersion returns resource version
func (p *AccessMonitoringRule) GetVersion() string {
	return p.Version
}

// GetKind returns resource kind
func (p *AccessMonitoringRule) GetKind() string {
	return p.Kind
}

// GetSubKind returns resource sub kind
func (p *AccessMonitoringRule) GetSubKind() string {
	return p.SubKind
}

// SetSubKind sets resource subkind
func (p *AccessMonitoringRule) SetSubKind(s string) {
	p.SubKind = s
}

// GetResourceID returns resource ID
func (p *AccessMonitoringRule) GetResourceID() int64 {
	return p.Metadata.ID
}

// SetResourceID sets resource ID
func (p *AccessMonitoringRule) SetResourceID(id int64) {
	p.Metadata.ID = id
}

// GetRevision returns the revision
func (p *AccessMonitoringRule) GetRevision() string {
	return p.Metadata.GetRevision()
}

// SetRevision sets the revision
func (p *AccessMonitoringRule) SetRevision(rev string) {
	p.Metadata.SetRevision(rev)
}

// GetMetadata returns object metadata
func (p *AccessMonitoringRule) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(p.Metadata)
}

// Expiry returns expiry time for the object
func (p *AccessMonitoringRule) Expiry() time.Time {
	return p.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (p *AccessMonitoringRule) SetExpiry(expires time.Time) {
	p.Metadata.SetExpiry(expires)
}

// GetName returns the name of the User
func (p *AccessMonitoringRule) GetName() string {
	return p.Metadata.Name
}

// SetName sets the name of the User
func (p *AccessMonitoringRule) SetName(e string) {
	p.Metadata.Name = e
}
