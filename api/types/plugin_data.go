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

	"github.com/gravitational/teleport/lib/utils"

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
	// CheckAndSetDefaults validates the plugin data
	// and supplies default values where appropriate.
	CheckAndSetDefaults() error
}

// NewPluginData configures a new PluginData instance associated
// with the supplied resource name (currently, this must be the
// name of an access request).
func NewPluginData(resourceName string, resourceKind string) (PluginData, error) {
	data := PluginDataV3{
		Kind:    KindPluginData,
		Version: V3,
		// If additional resource kinds become supported, make
		// this a parameter.
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

// SetTTL sets the resource time to live
func (r *PluginDataV3) SetTTL(clock Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// GetMetadata gets the resource metadata
func (r *PluginDataV3) GetMetadata() Metadata {
	return r.Metadata
}

// GetResourceID returns resource ID
func (r *PluginDataV3) GetResourceID() int64 {
	return r.Metadata.GetID()
}

// SetResourceID sets resource ID
func (r *PluginDataV3) SetResourceID(id int64) {
	r.Metadata.SetID(id)
}

func (r *PluginDataV3) String() string {
	return fmt.Sprintf("PluginData(kind=%s,resource=%s,entries=%d)", r.GetSubKind(), r.GetName(), len(r.Spec.Entries))
}

// CheckAndSetDefaults checks and sets default values for PluginData.
func (r *PluginDataV3) CheckAndSetDefaults() error {
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

// Equals compares two PluginDataEntries
func (d *PluginDataEntry) Equals(other *PluginDataEntry) bool {
	if other == nil {
		return false
	}
	if len(d.Data) != len(other.Data) {
		return false
	}
	for key, val := range d.Data {
		if other.Data[key] != val {
			return false
		}
	}
	return true
}

// PluginDataSpecSchema is JSON schema for PluginData
const PluginDataSpecSchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"entries": { "type":"object" }
	}
}`

// GetPluginDataSchema returns the full PluginDataSchema string
func GetPluginDataSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, PluginDataSpecSchema, DefaultDefinitions)
}

// PluginDataMarshaler implements marshal/unmarshal of PluginData implementations
type PluginDataMarshaler interface {
	MarshalPluginData(req PluginData, opts ...MarshalOption) ([]byte, error)
	UnmarshalPluginData(bytes []byte, opts ...MarshalOption) (PluginData, error)
}

type pluginDataMarshaler struct{}

func (m *pluginDataMarshaler) MarshalPluginData(data PluginData, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch r := data.(type) {
	case *PluginDataV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			cp := *r
			cp.SetResourceID(0)
			r = &cp
		}
		return utils.FastMarshal(r)
	default:
		return nil, trace.BadParameter("unrecognized plugin data type: %T", data)
	}
}

func (m *pluginDataMarshaler) UnmarshalPluginData(raw []byte, opts ...MarshalOption) (PluginData, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data PluginDataV3
	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(raw, &data); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := utils.UnmarshalWithSchema(GetPluginDataSchema(), &data, raw); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := data.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		data.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		data.SetExpiry(cfg.Expires)
	}
	return &data, nil
}

var pluginDataMarshalerInstance PluginDataMarshaler = &pluginDataMarshaler{}

// GetPluginDataMarshaler gets the global
func GetPluginDataMarshaler() PluginDataMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return pluginDataMarshalerInstance
}
