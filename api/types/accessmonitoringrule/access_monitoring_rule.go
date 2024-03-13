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
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types/header"
)

// AccessMonitoringRuleSubkind represents the type of the AccessMonitoringRule, e.g., access request, MDM etc.
type AccessMonitoringRuleSubkind string

const (
	// AccessMonitoringRuleSubkindUnknown is returned when no AccessMonitoringRule subkind matches.
	AccessMonitoringRuleSubkindUnknown AccessMonitoringRuleSubkind = ""
)

// AccessMonitoringRule represents a AccessMonitoringRule instance
type AccessMonitoringRule struct {
	// Metadata is the rules's metadata.
	Metadata header.Metadata `json:"metadata" yaml:"metadata"`
	// Kind is a resource kind
	Kind string `json:"kind" yaml:"kind"`
	// SubKind is an optional resource sub kind, used in some resources
	SubKind string `json:"sub_kind" yaml:"sub_kind"`
	// Version is the resource version
	Version string `json:"version" yaml:"version"`
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
		Metadata: metadata,
		Spec:     spec,
	}
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (amr *AccessMonitoringRule) GetMetadata() *headerv1.Metadata {
	md := amr.Metadata

	var expires *timestamppb.Timestamp
	if md.Expires.IsZero() {
		expires = timestamppb.New(md.Expires)
	}

	return &headerv1.Metadata{
		Name:        md.Name,
		Namespace:   defaults.Namespace,
		Description: md.Description,
		Labels:      md.Labels,
		Expires:     expires,
		Id:          md.ID,
		Revision:    md.Revision,
	}
}
