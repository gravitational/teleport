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
	"regexp"
	"sort"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
)

// Resource represents common properties for all resources.
type Resource interface {
	// GetKind returns resource kind
	GetKind() string
	// GetSubKind returns resource subkind
	GetSubKind() string
	// SetSubKind sets resource subkind
	SetSubKind(string)
	// GetVersion returns resource version
	GetVersion() string
	// GetName returns the name of the resource
	GetName() string
	// SetName sets the name of the resource
	SetName(string)
	// Expiry returns object expiry setting
	Expiry() time.Time
	// SetExpiry sets object expiry
	SetExpiry(time.Time)
	// GetMetadata returns object metadata
	GetMetadata() Metadata
	// GetResourceID returns resource ID
	GetResourceID() int64
	// SetResourceID sets resource ID
	SetResourceID(int64)
	// CheckAndSetDefaults validates the Resource and sets any empty fields to
	// default values.
	CheckAndSetDefaults() error
}

// ResourceWithSecrets includes additional properties which must
// be provided by resources which *may* contain secrets.
type ResourceWithSecrets interface {
	Resource
	// WithoutSecrets returns an instance of the resource which
	// has had all secrets removed.  If the current resource has
	// already had its secrets removed, this may be a no-op.
	WithoutSecrets() Resource
}

// ResourceWithOrigin provides information on the origin of the resource
// (defaults, config-file, dynamic).
type ResourceWithOrigin interface {
	Resource
	// Origin returns the origin value of the resource.
	Origin() string
	// SetOrigin sets the origin value of the resource.
	SetOrigin(string)
}

// ResourceWithLabels is a common interface for resources that have labels.
type ResourceWithLabels interface {
	// ResourceWithOrigin is the base resource interface.
	ResourceWithOrigin
	// GetAllLabels returns all resource's labels.
	GetAllLabels() map[string]string
}

// ResourcesWithLabels is a list of labeled resources.
type ResourcesWithLabels []ResourceWithLabels

// Find returns resource with the specified name or nil.
func (r ResourcesWithLabels) Find(name string) ResourceWithLabels {
	for _, resource := range r {
		if resource.GetName() == name {
			return resource
		}
	}
	return nil
}

// Len returns the slice length.
func (r ResourcesWithLabels) Len() int { return len(r) }

// Less compares resources by name.
func (r ResourcesWithLabels) Less(i, j int) bool { return r[i].GetName() < r[j].GetName() }

// Swap swaps two resources.
func (r ResourcesWithLabels) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

// GetVersion returns resource version
func (h *ResourceHeader) GetVersion() string {
	return h.Version
}

// GetResourceID returns resource ID
func (h *ResourceHeader) GetResourceID() int64 {
	return h.Metadata.ID
}

// SetResourceID sets resource ID
func (h *ResourceHeader) SetResourceID(id int64) {
	h.Metadata.ID = id
}

// GetName returns the name of the resource
func (h *ResourceHeader) GetName() string {
	return h.Metadata.Name
}

// SetName sets the name of the resource
func (h *ResourceHeader) SetName(v string) {
	h.Metadata.SetName(v)
}

// Expiry returns object expiry setting
func (h *ResourceHeader) Expiry() time.Time {
	return h.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (h *ResourceHeader) SetExpiry(t time.Time) {
	h.Metadata.SetExpiry(t)
}

// GetMetadata returns object metadata
func (h *ResourceHeader) GetMetadata() Metadata {
	return h.Metadata
}

// GetKind returns resource kind
func (h *ResourceHeader) GetKind() string {
	return h.Kind
}

// GetSubKind returns resource subkind
func (h *ResourceHeader) GetSubKind() string {
	return h.SubKind
}

// SetSubKind sets resource subkind
func (h *ResourceHeader) SetSubKind(s string) {
	h.SubKind = s
}

func (h *ResourceHeader) CheckAndSetDefaults() error {
	if h.Kind == "" {
		return trace.BadParameter("resource has an empty Kind field")
	}
	if h.Version == "" {
		return trace.BadParameter("resource has an empty Version field")
	}
	return trace.Wrap(h.Metadata.CheckAndSetDefaults())
}

// GetID returns resource ID
func (m *Metadata) GetID() int64 {
	return m.ID
}

// SetID sets resource ID
func (m *Metadata) SetID(id int64) {
	m.ID = id
}

// GetMetadata returns object metadata
func (m *Metadata) GetMetadata() Metadata {
	return *m
}

// GetName returns the name of the resource
func (m *Metadata) GetName() string {
	return m.Name
}

// SetName sets the name of the resource
func (m *Metadata) SetName(name string) {
	m.Name = name
}

// SetExpiry sets expiry time for the object
func (m *Metadata) SetExpiry(expires time.Time) {
	m.Expires = &expires
}

// Expiry returns object expiry setting.
func (m *Metadata) Expiry() time.Time {
	if m.Expires == nil {
		return time.Time{}
	}
	return *m.Expires
}

// Origin returns the origin value of the resource.
func (m *Metadata) Origin() string {
	if m.Labels == nil {
		return ""
	}
	return m.Labels[OriginLabel]
}

// SetOrigin sets the origin value of the resource.
func (m *Metadata) SetOrigin(origin string) {
	if m.Labels == nil {
		m.Labels = map[string]string{}
	}
	m.Labels[OriginLabel] = origin
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (m *Metadata) CheckAndSetDefaults() error {
	if m.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if m.Namespace == "" {
		m.Namespace = defaults.Namespace
	}

	// adjust expires time to UTC if it's set
	if m.Expires != nil {
		utils.UTC(m.Expires)
	}

	for key := range m.Labels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key: %q", key)
		}
	}

	// Check the origin value.
	if m.Origin() != "" {
		if !utils.SliceContainsStr(OriginValues, m.Origin()) {
			return trace.BadParameter("invalid origin value %q, must be one of %v", m.Origin(), OriginValues)
		}
	}

	return nil
}

// MatchLabels takes a map of labels and returns `true` if the resource has ALL
// of them.
func MatchLabels(resource ResourceWithLabels, labels map[string]string) bool {
	resourceLabels := resource.GetAllLabels()
	for name, value := range labels {
		if resourceLabels[name] != value {
			return false
		}
	}

	return true
}

// LabelPattern is a regexp that describes a valid label key
const LabelPattern = `^[a-zA-Z/.0-9_*-]+$`

var validLabelKey = regexp.MustCompile(LabelPattern)

// IsValidLabelKey checks if the supplied string matches the
// label key regexp.
func IsValidLabelKey(s string) bool {
	return validLabelKey.MatchString(s)
}

func compareStrByDir(a string, b string, dir SortDir) bool {
	if dir == SortDir_SORT_DIR_DESC {
		return a > b
	}
	return a < b
}

// filterableResourceFields defines resource fields that are filterable by resource type.
var filterableResourceFields = map[string]map[string]bool{
	ResourceFieldDescription: {
		KindApp:      true,
		KindDatabase: true,
	},
	ResourceFieldName: {
		KindApp:               true,
		KindDatabase:          true,
		KindKubernetesCluster: true,
		KindWindowsDesktop:    true,
	},
	// Below are resource type specific fields.
	ResourceFieldAddr: {
		KindNode:           true,
		KindWindowsDesktop: true,
	},
	ResourceFieldHostname: {
		KindNode: true,
	},
	ResourceFieldPublicAddr: {
		KindApp: true,
	},
	ResourceFieldType: {
		KindDatabase: true,
	},
}

// resourceSorter implements the Sort interface.
type resourceSorter struct {
	// resources is the data that will be sorted
	// and can only be one kind of resources.
	resources    []Resource
	resourceKind string
	sortBy       SortBy
	lessFn       func(i, j int) bool
	swapFn       func(i, j int)
}

// Resources returns a Sorter that sorts by resource type and a sort criteria.
// Only one type of resource can be in a list of resources.
// Call its Sort method to sort the data.
func Resources(resources []Resource, resourceKind string) *resourceSorter {
	return &resourceSorter{
		resources:    resources,
		resourceKind: resourceKind,
	}
}

// Len is part of sort.Interface.
func (s *resourceSorter) Len() int {
	return len(s.resources)
}

// Swap is part of sort.Interface.
func (s *resourceSorter) Swap(i, j int) {
	s.swapFn(i, j)
}

func (s *resourceSorter) swap(i, j int) {
	s.resources[i], s.resources[j] = s.resources[j], s.resources[i]
}

// Less is part of sort.Interface.
func (s *resourceSorter) Less(i, j int) bool {
	return s.lessFn(i, j)
}

// Sort sorts a list of resources according to the resource type and sort criteria.
func (s *resourceSorter) Sort(sortBy SortBy) error {
	s.sortBy = sortBy
	if len(s.resources) == 0 {
		return nil
	}

	if !filterableResourceFields[s.sortBy.Field][s.resourceKind] {
		return trace.NotImplemented("sorting by field %q is not supported", s.sortBy.Field)
	}

	// Try sorting by common resource fields first.
	if err := s.sortResources(); err == nil {
		return nil
	}

	// Then sort by resource kind specific fields.
	switch s.resourceKind {
	case KindApp:
		if err := s.sortApplications(); err != nil {
			return trace.Wrap(err)
		}
	case KindNode:
		if err := s.sortServers(); err != nil {
			return trace.Wrap(err)
		}
	case KindDatabase:
		if err := s.sortDatabases(); err != nil {
			return trace.Wrap(err)
		}
	case KindWindowsDesktop:
		if err := s.sortWindowsDesktop(); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.NotImplemented("resource type %q does not support sorting", s.resourceKind)
	}

	return nil
}

// sortResources sorts by common resource field.
func (s *resourceSorter) sortResources() error {
	s.swapFn = s.swap

	switch s.sortBy.Field {
	case ResourceFieldName:
		s.lessFn = func(i, j int) bool {
			return compareStrByDir(s.resources[i].GetName(), s.resources[j].GetName(), s.sortBy.Dir)
		}
	case ResourceFieldDescription:
		s.lessFn = func(i, j int) bool {
			return compareStrByDir(s.resources[i].GetMetadata().Description, s.resources[j].GetMetadata().Description, s.sortBy.Dir)
		}
	default:
		return trace.NotImplemented("sorting by field %q is not supported", s.sortBy.Field)
	}

	sort.Sort(s)
	return nil
}

// asApplications converts each resource into type Application.
func (s *resourceSorter) asApplications() ([]Application, error) {
	apps := make([]Application, len(s.resources))
	for i := range s.resources {
		app, ok := s.resources[i].(Application)
		if !ok {
			return nil, trace.BadParameter("expected types.Application, got: %T", s.resources[i])
		}
		apps[i] = app
	}
	return apps, nil
}

// sortApplications sorts by Application specific fields.
func (s *resourceSorter) sortApplications() error {
	apps, err := s.asApplications()
	if err != nil {
		return trace.Wrap(err)
	}

	s.swapFn = func(i, j int) {
		s.swap(i, j)
		apps[i], apps[j] = apps[j], apps[i]
	}

	switch s.sortBy.Field {
	case ResourceFieldPublicAddr:
		s.lessFn = func(i, j int) bool {
			return compareStrByDir(apps[i].GetPublicAddr(), apps[j].GetPublicAddr(), s.sortBy.Dir)
		}
	default:
		return trace.NotImplemented("sorting by field %q is not supported", s.sortBy.Field)
	}

	sort.Sort(s)
	return nil
}

// asServers converts each resource into type Server.
func (s *resourceSorter) asServers() ([]Server, error) {
	servers := make([]Server, len(s.resources))
	for i := range s.resources {
		server, ok := s.resources[i].(Server)
		if !ok {
			return nil, trace.BadParameter("expected types.Server, got: %T", s.resources[i])
		}
		servers[i] = server
	}
	return servers, nil
}

// sortServers sorts by Server specific fields.
func (s *resourceSorter) sortServers() error {
	servers, err := s.asServers()
	if err != nil {
		return trace.Wrap(err)
	}

	s.swapFn = func(i, j int) {
		s.swap(i, j)
		servers[i], servers[j] = servers[j], servers[i]
	}

	switch s.sortBy.Field {
	case ResourceFieldHostname:
		s.lessFn = func(i, j int) bool {
			return compareStrByDir(servers[i].GetHostname(), servers[j].GetHostname(), s.sortBy.Dir)
		}
	case ResourceFieldAddr:
		s.lessFn = func(i, j int) bool {
			return compareStrByDir(servers[i].GetAddr(), servers[j].GetAddr(), s.sortBy.Dir)
		}
	default:
		return trace.NotImplemented("sorting by field %q is not supported", s.sortBy.Field)
	}

	sort.Sort(s)
	return nil
}

// asDatabases converts each resource into type Database.
func (s *resourceSorter) asDatabases() ([]Database, error) {
	dbs := make([]Database, len(s.resources))
	for i := range s.resources {
		db, ok := s.resources[i].(Database)
		if !ok {
			return nil, trace.BadParameter("expected types.Database, got: %T", s.resources[i])
		}
		dbs[i] = db
	}
	return dbs, nil
}

// sortDatabases sorts by Database specific fields.
func (s *resourceSorter) sortDatabases() error {
	dbs, err := s.asDatabases()
	if err != nil {
		return trace.Wrap(err)
	}

	s.swapFn = func(i, j int) {
		s.swap(i, j)
		dbs[i], dbs[j] = dbs[j], dbs[i]
	}

	switch s.sortBy.Field {
	case ResourceFieldType:
		s.lessFn = func(i, j int) bool {
			return compareStrByDir(dbs[i].GetType(), dbs[j].GetType(), s.sortBy.Dir)
		}
	default:
		return trace.NotImplemented("sorting by field %q is not supported", s.sortBy.Field)
	}

	sort.Sort(s)
	return nil
}

// asWindowsDesktops converts each resource into type WindowsDesktop.
func (s *resourceSorter) asWindowsDesktops() ([]WindowsDesktop, error) {
	desktops := make([]WindowsDesktop, len(s.resources))
	for i := range s.resources {
		desktop, ok := s.resources[i].(WindowsDesktop)
		if !ok {
			return nil, trace.BadParameter("expected types.WindowsDesktop, got: %T", s.resources[i])
		}
		desktops[i] = desktop
	}
	return desktops, nil
}

// sortWindowsDesktop sorts by WindowsDesktop specific fields.
func (s *resourceSorter) sortWindowsDesktop() error {
	desktops, err := s.asWindowsDesktops()
	if err != nil {
		return trace.Wrap(err)
	}

	s.swapFn = func(i, j int) {
		s.swap(i, j)
		desktops[i], desktops[j] = desktops[j], desktops[i]
	}

	switch s.sortBy.Field {
	case ResourceFieldAddr:
		s.lessFn = func(i, j int) bool {
			return compareStrByDir(desktops[i].GetAddr(), desktops[j].GetAddr(), s.sortBy.Dir)
		}
	default:
		return trace.NotImplemented("sorting by field %q is not supported", s.sortBy.Field)
	}

	sort.Sort(s)
	return nil
}
