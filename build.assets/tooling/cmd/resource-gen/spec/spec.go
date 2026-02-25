/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package spec

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

var kindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var codePrefixPattern = regexp.MustCompile(`^[A-Z]{2,4}$`)

// PascalToSnake converts a PascalCase string to snake_case. It handles
// acronym runs correctly (e.g. "HTMLParser" → "html_parser", "GetURLValue" →
// "get_url_value", "AccessPolicy" → "access_policy").
func PascalToSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			// Insert underscore before this uppercase letter when:
			// 1. Not at the start, AND
			// 2. Either the previous char is lowercase, OR the next char is lowercase
			//    (the latter handles the end of an acronym run like "HTML|P").
			if i > 0 {
				prevLower := runes[i-1] >= 'a' && runes[i-1] <= 'z'
				nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
				if prevLower || nextLower {
					b.WriteByte('_')
				}
			}
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// StoragePattern identifies how a resource is keyed in backend storage.
type StoragePattern string

const (
	StoragePatternStandard  StoragePattern = "standard"
	StoragePatternSingleton StoragePattern = "singleton"
	StoragePatternScoped    StoragePattern = "scoped"
)

// ResourceSpec is the normalized configuration used by all generators.
type ResourceSpec struct {
	ServiceName string
	Kind        string // snake_case, e.g. "access_policy"
	KindPascal  string // PascalCase, e.g. "AccessPolicy" — preserved from service name

	Storage    StorageConfig
	Cache      CacheConfig
	Tctl       TctlConfig
	Audit      AuditConfig
	Hooks      HooksConfig
	Pagination PaginationConfig
	Operations OperationSet
}

// StorageConfig configures how resource keys are structured.
type StorageConfig struct {
	BackendPrefix string
	Pattern       StoragePattern
	SingletonName string
	ScopeBy       string
}

// CacheConfig controls generated cache behavior.
type CacheConfig struct {
	Enabled bool
	Indexes []string
}

// TctlConfig controls generated tctl behavior.
type TctlConfig struct {
	Description      string
	MFARequired      bool
	Columns          []string
	VerboseColumns   []string
	TimestampColumns []string
}

// AuditConfig controls audit event emission.
type AuditConfig struct {
	EmitOnCreate bool
	EmitOnUpdate bool
	EmitOnDelete bool
	EmitOnGet    bool
	CodePrefix   string
}

// HooksConfig controls lifecycle hook generation.
type HooksConfig struct {
	EnableLifecycleHooks bool
}

// PaginationConfig controls list operation pagination behavior.
type PaginationConfig struct {
	DefaultPageSize int32
	MaxPageSize     int32
}

// OperationSet tracks which operations are defined in proto RPCs.
type OperationSet struct {
	Get    bool
	List   bool
	Create bool
	Update bool
	Upsert bool
	Delete bool
}

// Validate checks required fields are present and coherent.
func (s ResourceSpec) Validate() error {
	if s.ServiceName == "" {
		return trace.BadParameter("service name is required")
	}
	if parts := strings.Split(s.ServiceName, "."); len(parts) < 4 {
		return trace.BadParameter("service name must be fully qualified with at least 4 parts (e.g. teleport.foo.v1.FooService), got %q", s.ServiceName)
	}
	if s.Kind == "" {
		return trace.BadParameter("kind is required")
	}
	if !kindPattern.MatchString(s.Kind) {
		return trace.BadParameter("kind must be lowercase and match [a-z][a-z0-9_]*, got %q", s.Kind)
	}
	if s.Storage.BackendPrefix == "" {
		return trace.BadParameter("backend prefix is required")
	}
	switch s.Storage.Pattern {
	case StoragePatternStandard:
	case StoragePatternSingleton:
		if s.Storage.SingletonName == "" {
			return trace.BadParameter("singleton storage requires fixed name")
		}
	case StoragePatternScoped:
		if s.Storage.ScopeBy == "" {
			return trace.BadParameter("scoped storage requires scope field")
		}
	default:
		return trace.BadParameter("storage pattern must be one of standard/singleton/scoped")
	}
	if s.Operations.Upsert && !s.Operations.Create {
		return trace.BadParameter("upsert requires create operation (upsert audit delegates to create)")
	}
	if len(s.Cache.Indexes) > 0 && (len(s.Cache.Indexes) != 1 || s.Cache.Indexes[0] != "metadata.name") {
		return trace.NotImplemented("custom cache.indexes are not yet supported by resource-gen (only [\"metadata.name\"] is supported)")
	}
	if s.Pagination.DefaultPageSize <= 0 {
		return trace.BadParameter("default page size must be > 0")
	}
	if s.Pagination.MaxPageSize <= 0 {
		return trace.BadParameter("max page size must be > 0")
	}
	if s.Pagination.MaxPageSize < s.Pagination.DefaultPageSize {
		return trace.BadParameter("max page size must be >= default page size")
	}
	if s.Storage.Pattern == StoragePatternSingleton && s.Operations.List {
		return trace.BadParameter("singleton storage does not support List operations")
	}
	if s.Operations.List && !s.Operations.Get {
		return trace.BadParameter("List operation requires Get (tctl get handler requires both)")
	}
	if s.Audit.EmitOnGet && !s.Operations.Get {
		return trace.BadParameter("audit.emit_on_get requires Get operation")
	}
	if (s.Operations.Update && s.Audit.EmitOnUpdate || s.Operations.Upsert && s.Audit.EmitOnUpdate) && !s.Operations.Get {
		return trace.BadParameter("audit.emit_on_update requires Get operation (audit pre-fetches old resource via reader.Get)")
	}
	if s.Cache.Enabled && !s.Operations.List {
		return trace.BadParameter("cache.enabled requires List operation (cache fetcher uses List to populate)")
	}
	if s.Cache.Enabled && s.Storage.Pattern == StoragePatternScoped {
		return trace.NotImplemented("cache is not yet supported for scoped resources")
	}
	if s.Operations.Upsert && s.Audit.EmitOnUpdate && !s.Audit.EmitOnCreate {
		return trace.BadParameter("upsert with audit.emit_on_update requires audit.emit_on_create (upsert audit delegates to create when resource is new)")
	}

	hasAudit := s.Audit.EmitOnCreate || s.Audit.EmitOnUpdate || s.Audit.EmitOnDelete || s.Audit.EmitOnGet
	if hasAudit && s.Audit.CodePrefix == "" {
		return trace.BadParameter("audit.code_prefix is required when any audit event emission is enabled")
	}
	if s.Audit.CodePrefix != "" && !codePrefixPattern.MatchString(s.Audit.CodePrefix) {
		return trace.BadParameter("audit.code_prefix must be 2-4 uppercase ASCII characters, got %q", s.Audit.CodePrefix)
	}

	return nil
}
