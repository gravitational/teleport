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
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/compare"
	"github.com/gravitational/teleport/api/utils"
)

var (
	_ compare.IsEqual[*ResourceHeader] = (*ResourceHeader)(nil)
	_ compare.IsEqual[*Metadata]       = (*Metadata)(nil)
)

// Resource represents common properties for all resources.
//
// Please avoid adding new uses of Resource in the codebase. Instead, consider
// using concrete proto types directly or a manually declared subset of the
// Resource153 interface for new-style resources.
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
	// Deprecated: use GetRevision instead
	GetResourceID() int64
	// SetResourceID sets resource ID
	// Deprecated: use SetRevision instead
	SetResourceID(int64)
	// GetRevision returns the revision
	GetRevision() string
	// SetRevision sets the revision
	SetRevision(string)
}

// IsSystemResource checks to see if the given resource is considered
// part of the teleport system, as opposed to some user created resource
// or preset.
func IsSystemResource(r Resource) bool {
	metadata := r.GetMetadata()
	if t, ok := metadata.Labels[TeleportInternalResourceType]; ok {
		return t == SystemResource
	}
	return false
}

// GetName fetches the name of the supplied resource. Useful when sorting lists
// of resources or building maps, etc.
func GetName[R Resource](r R) string {
	return r.GetName()
}

// ResourceDetails includes details about the resource
type ResourceDetails struct {
	Hostname     string
	FriendlyName string
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
	// GetLabel retrieves the label with the provided key.
	GetLabel(key string) (value string, ok bool)
	// GetAllLabels returns all resource's labels.
	GetAllLabels() map[string]string
	// GetStaticLabels returns the resource's static labels.
	GetStaticLabels() map[string]string
	// SetStaticLabels sets the resource's static labels.
	SetStaticLabels(sl map[string]string)
	// MatchSearch goes through select field values of a resource
	// and tries to match against the list of search values.
	MatchSearch(searchValues []string) bool
}

// EnrichedResource is a [ResourceWithLabels] wrapped with
// additional user-specific information.
type EnrichedResource struct {
	// ResourceWithLabels is the underlying resource.
	ResourceWithLabels
	// Logins that the user is allowed to access the above resource with.
	Logins []string
	// RequiresRequest is true if a resource is being returned to the user but requires
	// an access request to access. This is done during `ListUnifiedResources` when
	// searchAsRoles is true
	RequiresRequest bool
}

// ResourcesWithLabels is a list of labeled resources.
type ResourcesWithLabels []ResourceWithLabels

// ResourcesWithLabelsMap is like ResourcesWithLabels, but a map from resource name to its value.
type ResourcesWithLabelsMap map[string]ResourceWithLabels

// ToMap returns these databases as a map keyed by database name.
func (r ResourcesWithLabels) ToMap() ResourcesWithLabelsMap {
	rm := make(ResourcesWithLabelsMap, len(r))

	// there may be duplicate resources in the input list.
	// by iterating from end to start, the first resource of given name wins.
	for i := len(r) - 1; i >= 0; i-- {
		resource := r[i]
		rm[resource.GetName()] = resource
	}

	return rm
}

// Len returns the slice length.
func (r ResourcesWithLabels) Len() int { return len(r) }

// Less compares resources by name.
func (r ResourcesWithLabels) Less(i, j int) bool { return r[i].GetName() < r[j].GetName() }

// Swap swaps two resources.
func (r ResourcesWithLabels) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

// AsAppServers converts each resource into type AppServer.
func (r ResourcesWithLabels) AsAppServers() ([]AppServer, error) {
	apps := make([]AppServer, 0, len(r))
	for _, resource := range r {
		app, ok := resource.(AppServer)
		if !ok {
			return nil, trace.BadParameter("expected types.AppServer, got: %T", resource)
		}
		apps = append(apps, app)
	}
	return apps, nil
}

// AsServers converts each resource into type Server.
func (r ResourcesWithLabels) AsServers() ([]Server, error) {
	servers := make([]Server, 0, len(r))
	for _, resource := range r {
		server, ok := resource.(Server)
		if !ok {
			return nil, trace.BadParameter("expected types.Server, got: %T", resource)
		}
		servers = append(servers, server)
	}
	return servers, nil
}

// AsDatabases converts each resource into type Database.
func (r ResourcesWithLabels) AsDatabases() ([]Database, error) {
	dbs := make([]Database, 0, len(r))
	for _, resource := range r {
		db, ok := resource.(Database)
		if !ok {
			return nil, trace.BadParameter("expected types.Database, got: %T", resource)
		}
		dbs = append(dbs, db)
	}
	return dbs, nil
}

// AsDatabaseServers converts each resource into type DatabaseServer.
func (r ResourcesWithLabels) AsDatabaseServers() ([]DatabaseServer, error) {
	dbs := make([]DatabaseServer, 0, len(r))
	for _, resource := range r {
		db, ok := resource.(DatabaseServer)
		if !ok {
			return nil, trace.BadParameter("expected types.DatabaseServer, got: %T", resource)
		}
		dbs = append(dbs, db)
	}
	return dbs, nil
}

// AsDatabaseServices converts each resource into type DatabaseService.
func (r ResourcesWithLabels) AsDatabaseServices() ([]DatabaseService, error) {
	services := make([]DatabaseService, len(r))
	for i, resource := range r {
		dbService, ok := resource.(DatabaseService)
		if !ok {
			return nil, trace.BadParameter("expected types.DatabaseService, got: %T", resource)
		}
		services[i] = dbService
	}
	return services, nil
}

// AsWindowsDesktops converts each resource into type WindowsDesktop.
func (r ResourcesWithLabels) AsWindowsDesktops() ([]WindowsDesktop, error) {
	desktops := make([]WindowsDesktop, 0, len(r))
	for _, resource := range r {
		desktop, ok := resource.(WindowsDesktop)
		if !ok {
			return nil, trace.BadParameter("expected types.WindowsDesktop, got: %T", resource)
		}
		desktops = append(desktops, desktop)
	}
	return desktops, nil
}

// AsWindowsDesktopServices converts each resource into type WindowsDesktop.
func (r ResourcesWithLabels) AsWindowsDesktopServices() ([]WindowsDesktopService, error) {
	desktopServices := make([]WindowsDesktopService, 0, len(r))
	for _, resource := range r {
		desktopService, ok := resource.(WindowsDesktopService)
		if !ok {
			return nil, trace.BadParameter("expected types.WindowsDesktopService, got: %T", resource)
		}
		desktopServices = append(desktopServices, desktopService)
	}
	return desktopServices, nil
}

// AsKubeClusters converts each resource into type KubeCluster.
func (r ResourcesWithLabels) AsKubeClusters() ([]KubeCluster, error) {
	clusters := make([]KubeCluster, 0, len(r))
	for _, resource := range r {
		cluster, ok := resource.(KubeCluster)
		if !ok {
			return nil, trace.BadParameter("expected types.KubeCluster, got: %T", resource)
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

// AsKubeServers converts each resource into type KubeServer.
func (r ResourcesWithLabels) AsKubeServers() ([]KubeServer, error) {
	servers := make([]KubeServer, 0, len(r))
	for _, resource := range r {
		server, ok := resource.(KubeServer)
		if !ok {
			return nil, trace.BadParameter("expected types.KubeServer, got: %T", resource)
		}
		servers = append(servers, server)
	}
	return servers, nil
}

// AsUserGroups converts each resource into type UserGroup.
func (r ResourcesWithLabels) AsUserGroups() ([]UserGroup, error) {
	userGroups := make([]UserGroup, 0, len(r))
	for _, resource := range r {
		userGroup, ok := resource.(UserGroup)
		if !ok {
			return nil, trace.BadParameter("expected types.UserGroup, got: %T", resource)
		}
		userGroups = append(userGroups, userGroup)
	}
	return userGroups, nil
}

// GetVersion returns resource version
func (h *ResourceHeader) GetVersion() string {
	return h.Version
}

// GetResourceID returns resource ID
// Deprecated: Use GetRevision instead.
func (h *ResourceHeader) GetResourceID() int64 {
	return h.Metadata.ID
}

// SetResourceID sets resource ID
// Deprecated: Use SetRevision instead.
func (h *ResourceHeader) SetResourceID(id int64) {
	h.Metadata.ID = id
}

// GetRevision returns the revision
func (h *ResourceHeader) GetRevision() string {
	return h.Metadata.GetRevision()
}

// SetRevision sets the revision
func (h *ResourceHeader) SetRevision(rev string) {
	h.Metadata.SetRevision(rev)
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

// Origin returns the origin value of the resource.
func (h *ResourceHeader) Origin() string {
	return h.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (h *ResourceHeader) SetOrigin(origin string) {
	h.Metadata.SetOrigin(origin)
}

// GetStaticLabels returns the static labels for the resource.
func (h *ResourceHeader) GetStaticLabels() map[string]string {
	return h.Metadata.Labels
}

// SetStaticLabels sets the static labels for the resource.
func (h *ResourceHeader) SetStaticLabels(sl map[string]string) {
	h.Metadata.Labels = sl
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (h *ResourceHeader) GetLabel(key string) (value string, ok bool) {
	v, ok := h.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns all labels from the resource..
func (h *ResourceHeader) GetAllLabels() map[string]string {
	return h.Metadata.Labels
}

// IsEqual determines if two resource header resources are equivalent to one another.
func (h *ResourceHeader) IsEqual(other *ResourceHeader) bool {
	return deriveTeleportEqualResourceHeader(h, other)
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

// GetRevision returns the revision
func (m *Metadata) GetRevision() string {
	return m.Revision
}

// SetRevision sets the revision
func (m *Metadata) SetRevision(rev string) {
	m.Revision = rev
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

// IsEqual determines if two metadata resources are equivalent to one another.
func (m *Metadata) IsEqual(other *Metadata) bool {
	return deriveTeleportEqualMetadata(m, other)
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
		if !slices.Contains(OriginValues, m.Origin()) {
			return trace.BadParameter("invalid origin value %q, must be one of %v", m.Origin(), OriginValues)
		}
	}

	return nil
}

// MatchLabels takes a map of labels and returns `true` if the resource has ALL
// of them.
func MatchLabels(resource ResourceWithLabels, labels map[string]string) bool {
	for key, value := range labels {
		if v, ok := resource.GetLabel(key); !ok || v != value {
			return false
		}
	}

	return true
}

// MatchKinds takes an array of strings that represent a Kind and
// returns true if the resource's kind matches any item in the given array.
func MatchKinds(resource ResourceWithLabels, kinds []string) bool {
	if len(kinds) == 0 {
		return true
	}
	resourceKind := resource.GetKind()
	switch resourceKind {
	case KindApp, KindSAMLIdPServiceProvider:
		return slices.Contains(kinds, KindApp)
	default:
		return slices.Contains(kinds, resourceKind)
	}
}

// IsValidLabelKey checks if the supplied string matches the
// label key regexp.
func IsValidLabelKey(s string) bool {
	return common.IsValidLabelKey(s)
}

// MatchSearch goes through select field values from a resource
// and tries to match against the list of search values, ignoring case and order.
// Returns true if all search vals were matched (or if nil search vals).
// Returns false if no or partial match (or nil field values).
func MatchSearch(fieldVals []string, searchVals []string, customMatch func(val string) bool) bool {
Outer:
	for _, searchV := range searchVals {
		// Iterate through field values to look for a match.
		for _, fieldV := range fieldVals {
			if containsFold(fieldV, searchV) {
				continue Outer
			}
		}

		if customMatch != nil && customMatch(searchV) {
			continue
		}

		// When no fields matched a value, prematurely end if we can.
		return false
	}

	return true
}

// containsFold is a case-insensitive alternative to strings.Contains, used to help avoid excess allocations during searches.
func containsFold(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}

	n := len(s) - len(substr)

	for i := 0; i <= n; i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return true
		}
	}

	return false
}

func stringCompare(a string, b string, isDesc bool) bool {
	if isDesc {
		return a > b
	}
	return a < b
}

var kindsOrder = []string{
	"app", "db", "windows_desktop", "kube_cluster", "node",
}

// unifiedKindCompare compares two resource kinds and returns true if a is less than b.
// Note that it's not just a simple string comparison, since the UI names these
// kinds slightly differently, and hence uses a different alphabetical order for
// them.
//
// If resources are of the same kind, this function falls back to comparing
// their unified names.
func unifiedKindCompare(a, b ResourceWithLabels, isDesc bool) bool {
	ak := a.GetKind()
	bk := b.GetKind()

	if ak == bk {
		return unifiedNameCompare(a, b, isDesc)
	}

	ia := slices.Index(kindsOrder, ak)
	ib := slices.Index(kindsOrder, bk)
	if ia < 0 && ib < 0 {
		// Fallback for a case of two unknown resources.
		return stringCompare(ak, bk, isDesc)
	}
	if isDesc {
		return ia > ib
	}
	return ia < ib
}

func unifiedNameCompare(a ResourceWithLabels, b ResourceWithLabels, isDesc bool) bool {
	var nameA, nameB string
	switch r := a.(type) {
	case AppServer:
		nameA = r.GetApp().GetName()
	case DatabaseServer:
		nameA = r.GetDatabase().GetName()
	case KubeServer:
		nameA = r.GetCluster().GetName()
	case Server:
		nameA = r.GetHostname()
	default:
		nameA = a.GetName()
	}

	switch r := b.(type) {
	case AppServer:
		nameB = r.GetApp().GetName()
	case DatabaseServer:
		nameB = r.GetDatabase().GetName()
	case KubeServer:
		nameB = r.GetCluster().GetName()
	case Server:
		nameB = r.GetHostname()
	default:
		nameB = a.GetName()
	}

	return stringCompare(strings.ToLower(nameA), strings.ToLower(nameB), isDesc)
}

func (r ResourcesWithLabels) SortByCustom(by SortBy) error {
	isDesc := by.IsDesc
	switch by.Field {
	case ResourceMetadataName:
		sort.SliceStable(r, func(i, j int) bool {
			return unifiedNameCompare(r[i], r[j], isDesc)
		})
	case ResourceKind:
		sort.SliceStable(r, func(i, j int) bool {
			return unifiedKindCompare(r[i], r[j], isDesc)
		})
	default:
		return trace.NotImplemented("sorting by field %q for unified resource %q is not supported", by.Field, KindUnifiedResource)
	}
	return nil
}

// ListResourcesResponse describes a non proto response to ListResources.
type ListResourcesResponse struct {
	// Resources is a list of resource.
	Resources []ResourceWithLabels
	// NextKey is the next key to use as a starting point.
	NextKey string
	// TotalCount is the total number of resources available as a whole.
	TotalCount int
}

// ValidateResourceName validates a resource name using a given regexp.
func ValidateResourceName(validationRegex *regexp.Regexp, name string) error {
	if validationRegex.MatchString(name) {
		return nil
	}
	return trace.BadParameter(
		"%q does not match regex used for validation %q",
		name, validationRegex.String(),
	)
}

// FriendlyName will return the friendly name for a resource if it has one. Otherwise, it
// will return an empty string.
func FriendlyName(resource ResourceWithLabels) string {
	// Right now, only resources sourced from Okta and nodes have friendly names.
	if resource.Origin() == OriginOkta {
		if appName, ok := resource.GetLabel(OktaAppNameLabel); ok {
			return appName
		} else if groupName, ok := resource.GetLabel(OktaGroupNameLabel); ok {
			return groupName
		} else if roleName, ok := resource.GetLabel(OktaRoleNameLabel); ok {
			return roleName
		}
		return resource.GetMetadata().Description
	}

	if hn, ok := resource.(interface{ GetHostname() string }); ok {
		return hn.GetHostname()
	}

	return ""
}

// GetOrigin returns the value set for the [OriginLabel].
// If the label is missing, an empty string is returned.
//
// Works for both [ResourceWithOrigin] and [ResourceMetadata] instances.
func GetOrigin(v any) (string, error) {
	switch r := v.(type) {
	case ResourceWithOrigin:
		return r.Origin(), nil
	case ResourceMetadata:
		meta := r.GetMetadata()
		if meta.Labels == nil {
			return "", nil
		}
		return meta.Labels[OriginLabel], nil
	}
	return "", trace.BadParameter("unable to determine origin from resource of type %T", v)
}

// GetKind returns the kind, if one can be obtained, otherwise
// an empty string is returned.
//
// Works for both [Resource] and [ResourceMetadata] instances.
func GetKind(v any) (string, error) {
	type kinder interface {
		GetKind() string
	}
	if k, ok := v.(kinder); ok {
		return k.GetKind(), nil
	}
	return "", trace.BadParameter("unable to determine kind from resource of type %T", v)
}

// GetRevision returns the revision, if one can be obtained, otherwise
// an empty string is returned.
//
// Works for both [Resource] and [ResourceMetadata] instances.
func GetRevision(v any) (string, error) {
	switch r := v.(type) {
	case Resource:
		return r.GetRevision(), nil
	case ResourceMetadata:
		return r.GetMetadata().Revision, nil
	}
	return "", trace.BadParameter("unable to determine revision from resource of type %T", v)
}

// SetRevision updates the revision if v supports the concept of revisions.
//
// Works for both [Resource] and [ResourceMetadata] instances.
func SetRevision(v any, revision string) error {
	switch r := v.(type) {
	case Resource:
		r.SetRevision(revision)
		return nil
	case ResourceMetadata:
		r.GetMetadata().Revision = revision
		return nil
	}
	return trace.BadParameter("unable to set revision on resource of type %T", v)
}

// GetExpiry returns the expiration, if one can be obtained, otherwise returns
// an empty time `time.Time{}`, which is equivalent to no expiry.
//
// Works for both [Resource] and [ResourceMetadata] instances.
func GetExpiry(v any) (time.Time, error) {
	switch r := v.(type) {
	case Resource:
		return r.Expiry(), nil
	case ResourceMetadata:
		// ResourceMetadata uses *timestamppb.Timestamp instead of time.Time. The zero value for this type is 01/01/1970.
		// This is a problem for resources without explicit expiry set: they'd become obsolete on creation.
		// For this reason, we check for nil expiry explicitly, and default it to time.Time{}.
		exp := r.GetMetadata().GetExpires()
		if exp == nil {
			return time.Time{}, nil
		}
		return exp.AsTime(), nil
	}
	return time.Time{}, trace.BadParameter("unable to determine expiry from resource of type %T", v)
}

// GetResourceID returns the id, if one can be obtained, otherwise returns
// zero.
//
// Works for both [Resource] and [ResourceMetadata] instances.
//
// Deprecated: GetRevision should be used instead.
func GetResourceID(v any) (int64, error) {
	switch r := v.(type) {
	case Resource:
		//nolint:staticcheck // SA1019. Added for backward compatibility.
		return r.GetResourceID(), nil
	case ResourceMetadata:
		//nolint:staticcheck // SA1019. Added for backward compatibility.
		return r.GetMetadata().Id, nil
	}
	return 0, trace.BadParameter("unable to determine resource ID from resource of type %T", v)
}
