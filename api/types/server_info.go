/*
Copyright 2023 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/utils"
)

// ServerInfo represents info that should be applied to joining Nodes.
type ServerInfo interface {
	// ResourceWithLabels provides common resource headers
	ResourceWithLabels
	// GetNewLabels gets the labels to apply to matched Nodes.
	GetNewLabels() map[string]string
	// SetNewLabels sets the labels to apply to matched Nodes.
	SetNewLabels(map[string]string)
}

// NewServerInfo creates an instance of ServerInfo.
func NewServerInfo(meta Metadata, spec ServerInfoSpecV1) (ServerInfo, error) {
	si := &ServerInfoV1{
		Metadata: meta,
		Spec:     spec,
	}
	if err := si.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return si, nil
}

// GetKind returns resource kind
func (s *ServerInfoV1) GetKind() string {
	return s.Kind
}

// GetSubKind returns resource subkind
func (s *ServerInfoV1) GetSubKind() string {
	return s.SubKind
}

// SetSubKind sets resource subkind
func (s *ServerInfoV1) SetSubKind(subkind string) {
	s.SubKind = subkind
}

// GetVersion returns resource version
func (s *ServerInfoV1) GetVersion() string {
	return s.Version
}

// GetName returns the name of the resource
func (s *ServerInfoV1) GetName() string {
	return s.Metadata.Name
}

// SetName sets the name of the resource
func (s *ServerInfoV1) SetName(name string) {
	s.Metadata.Name = name
}

// Expiry returns object expiry setting
func (s *ServerInfoV1) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (s *ServerInfoV1) SetExpiry(expiry time.Time) {
	s.Metadata.SetExpiry(expiry)
}

// GetMetadata returns object metadata
func (s *ServerInfoV1) GetMetadata() Metadata {
	return s.Metadata
}

// GetResourceID returns resource ID
func (s *ServerInfoV1) GetResourceID() int64 {
	return s.Metadata.ID
}

// SetResourceID sets resource ID
func (s *ServerInfoV1) SetResourceID(id int64) {
	s.Metadata.ID = id
}

// Origin returns the origin value of the resource.
func (s *ServerInfoV1) Origin() string {
	return s.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (s *ServerInfoV1) SetOrigin(o string) {
	s.Metadata.SetOrigin(o)
}

// GetLabel retrieves the label with the provided key.
func (s *ServerInfoV1) GetLabel(key string) (string, bool) {
	value, ok := s.Metadata.Labels[key]
	return value, ok
}

// GetAllLabels returns all resource's labels.
func (s *ServerInfoV1) GetAllLabels() map[string]string {
	return s.Metadata.Labels
}

// GetStaticLabels returns the resource's static labels.
func (s *ServerInfoV1) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the resource's static labels.
func (s *ServerInfoV1) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// MatchSearch goes through select field values of a resource
// and tries to match against the list of search values.
func (s *ServerInfoV1) MatchSearch(searchValues []string) bool {
	fieldVals := append(
		utils.MapToStrings(s.GetAllLabels()),
		s.GetName(),
	)
	return MatchSearch(fieldVals, searchValues, nil)
}

// GetNewLabels gets the labels to apply to matched Nodes.
func (s *ServerInfoV1) GetNewLabels() map[string]string {
	return s.Spec.NewLabels
}

// SetNewLabels sets the labels to apply to matched Nodes.
func (s *ServerInfoV1) SetNewLabels(labels map[string]string) {
	s.Spec.NewLabels = labels
}

func (s *ServerInfoV1) setStaticFields() {
	s.Kind = KindServerInfo
	s.Version = V1
}

// CheckAndSetDefaults validates the Resource and sets any empty fields to
// default values.
func (s *ServerInfoV1) CheckAndSetDefaults() error {
	s.setStaticFields()
	return trace.Wrap(s.Metadata.CheckAndSetDefaults())
}

// GetServerInfoName gets the name of the ServerInfo generated for a discovered
// EC2 instance with this account ID and instance ID.
func (a *AWSInfo) GetServerInfoName() string {
	return fmt.Sprintf("aws-%v-%v", a.AccountID, a.InstanceID)
}
