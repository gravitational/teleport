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

// PluginData is used by plugins to store per-resource state.  An instance of PluginData
// corresponds to a resource which may be managed by one or more plugins.  Data is stored
// as a mapping of the form `plugin -> key -> val`, effectively giving each plugin its own
// key-value store.  Importantly, an instance of PluginData can only be created for a resource
// which currently exist, and automatically expires shortly after the corresponding resource.
// Currently, only the AccessRequest resource is supported.
type PluginData interface {
	Resource
	// Entries gets all entries.
	Entries() map[string]*PluginDataEntry
	// Update attempts to apply an update.
	Update(params PluginDataUpdateParams) error
}

// NewPluginData configures a new PluginData instance associated
// with the supplied resource name (currently, this must be the
// name of an access request).
func NewPluginData(resourceName string, resourceKind string) (PluginData, error) {
	data := PluginDataV3{
		SubKind: resourceKind,
		Metadata: Metadata{
			Name: resourceName,
		},
		Spec: PluginDataSpecV3{
			Entries: make(map[string]*PluginDataEntry),
		},
	}
	if err := data.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetKind returns resource kind
func (r *PluginDataV3) GetKind() string {
	return r.Kind
}

// GetSubKind returns resource subkind
func (r *PluginDataV3) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *PluginDataV3) SetSubKind(subKind string) {
	r.SubKind = subKind
}

// GetVersion gets resource version
func (r *PluginDataV3) GetVersion() string {
	return r.Version
}

// GetName gets resource name
func (r *PluginDataV3) GetName() string {
	return r.Metadata.Name
}

// SetName sets resource name
func (r *PluginDataV3) SetName(name string) {
	r.Metadata.Name = name
}

// Expiry returns object expiry setting
func (r *PluginDataV3) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (r *PluginDataV3) SetExpiry(expiry time.Time) {
	r.Metadata.SetExpiry(expiry)
}

// GetMetadata gets the resource metadata
func (r *PluginDataV3) GetMetadata() Metadata {
	return r.Metadata
}

// GetRevision returns the revision
func (r *PluginDataV3) GetRevision() string {
	return r.Metadata.GetRevision()
}

// SetRevision sets the revision
func (r *PluginDataV3) SetRevision(rev string) {
	r.Metadata.SetRevision(rev)
}

func (r *PluginDataV3) String() string {
	return fmt.Sprintf("PluginData(kind=%s,resource=%s,entries=%d)", r.GetSubKind(), r.GetName(), len(r.Spec.Entries))
}

// setStaticFields sets static resource header and metadata fields.
func (r *PluginDataV3) setStaticFields() {
	r.Kind = KindPluginData
	r.Version = V3
}

// CheckAndSetDefaults checks and sets default values for PluginData.
func (r *PluginDataV3) CheckAndSetDefaults() error {
	r.setStaticFields()
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if r.SubKind == "" {
		return trace.BadParameter("plugin data missing subkind")
	}

	return nil
}

// Entries returns the PluginData entires
func (r *PluginDataV3) Entries() map[string]*PluginDataEntry {
	if r.Spec.Entries == nil {
		r.Spec.Entries = make(map[string]*PluginDataEntry)
	}
	return r.Spec.Entries
}

// Update updates the PluginData
func (r *PluginDataV3) Update(params PluginDataUpdateParams) error {
	// See #3286 for a complete discussion of the design constraints at play here.

	if params.Kind != r.GetSubKind() {
		return trace.BadParameter("resource kind mismatch in update params")
	}

	if params.Resource != r.GetName() {
		return trace.BadParameter("resource name mismatch in update params")
	}

	// If expectations were given, ensure that they are met before continuing
	if params.Expect != nil {
		if err := r.checkExpectations(params.Plugin, params.Expect); err != nil {
			return trace.Wrap(err)
		}
	}
	// Ensure that Entries has been initialized
	if r.Spec.Entries == nil {
		r.Spec.Entries = make(map[string]*PluginDataEntry, 1)
	}
	// Ensure that the specific Plugin has been initialized
	if r.Spec.Entries[params.Plugin] == nil {
		r.Spec.Entries[params.Plugin] = &PluginDataEntry{
			Data: make(map[string]string, len(params.Set)),
		}
	}
	entry := r.Spec.Entries[params.Plugin]
	for key, val := range params.Set {
		// Keys which are explicitly set to the empty string are
		// treated as DELETE operations.
		if val == "" {
			delete(entry.Data, key)
			continue
		}
		entry.Data[key] = val
	}
	// Its possible that this update was simply clearing all data;
	// if that is the case, remove the entry.
	if len(entry.Data) == 0 {
		delete(r.Spec.Entries, params.Plugin)
	}
	return nil
}

// checkExpectations verifies that the data for `plugin` matches the expected
// state described by `expect`.  This function implements the behavior of the
// `PluginDataUpdateParams.Expect` mapping.
func (r *PluginDataV3) checkExpectations(plugin string, expect map[string]string) error {
	var entry *PluginDataEntry
	if r.Spec.Entries != nil {
		entry = r.Spec.Entries[plugin]
	}
	if entry == nil {
		// If no entry currently exists, then the only expectation that can
		// match is one which only specifies fields which shouldn't exist.
		for key, val := range expect {
			if val != "" {
				return trace.CompareFailed("expectations not met for field %q", key)
			}
		}
		return nil
	}
	for key, val := range expect {
		if entry.Data[key] != val {
			return trace.CompareFailed("expectations not met for field %q", key)

		}
	}
	return nil
}

// Match returns true if the PluginData given matches the filter
func (f *PluginDataFilter) Match(data PluginData) bool {
	if f.Kind != "" && f.Kind != data.GetSubKind() {
		return false
	}
	if f.Resource != "" && f.Resource != data.GetName() {
		return false
	}
	if f.Plugin != "" {
		if _, ok := data.Entries()[f.Plugin]; !ok {
			return false
		}
	}
	return true
}
