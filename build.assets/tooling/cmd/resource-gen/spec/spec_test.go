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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpecZeroValueInvalid(t *testing.T) {
	var s ResourceSpec
	require.Error(t, s.Validate())
}

func TestSpecValidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    ResourceSpec
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "valid",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage: StorageConfig{
					BackendPrefix: "foos",
					Pattern:       StoragePatternStandard,
				},
				Pagination: PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
			},
			wantErr: require.NoError,
		},
		{
			name: "invalid pattern",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage: StorageConfig{
					BackendPrefix: "foos",
					Pattern:       "invalid",
				},
				Pagination: PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
			},
			wantErr: require.Error,
		},
		{
			name: "scoped missing scope field",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage: StorageConfig{
					BackendPrefix: "foos",
					Pattern:       StoragePatternScoped,
				},
				Pagination: PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
			},
			wantErr: require.Error,
		},
		{
			name: "upsert without create",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage: StorageConfig{
					BackendPrefix: "foos",
					Pattern:       StoragePatternStandard,
				},
				Pagination: PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Operations: OperationSet{Upsert: true, Update: true},
			},
			wantErr: require.Error,
		},
		{
			name: "emit_on_get is valid",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Audit:       AuditConfig{EmitOnGet: true, CodePrefix: "FO"},
				Operations:  OperationSet{Get: true},
			},
			wantErr: require.NoError,
		},
		{
			name: "custom cache indexes not implemented",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Cache:       CacheConfig{Indexes: []string{"metadata.name", "spec.flavor"}},
			},
			wantErr: require.Error,
		},
		{
			name: "default cache index is fine",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Cache:       CacheConfig{Indexes: []string{"metadata.name"}},
			},
			wantErr: require.NoError,
		},
		{
			name: "invalid pagination",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage: StorageConfig{
					BackendPrefix: "foos",
					Pattern:       StoragePatternStandard,
				},
				Pagination: PaginationConfig{DefaultPageSize: 1000, MaxPageSize: 200},
			},
			wantErr: require.Error,
		},
		{
			name: "service name too short",
			spec: ResourceSpec{
				ServiceName: "foo.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
			},
			wantErr: require.Error,
		},
		{
			name: "kind with uppercase",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "Foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
			},
			wantErr: require.Error,
		},
		{
			name: "kind with hyphen",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo-bar",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
			},
			wantErr: require.Error,
		},
		{
			name: "singleton with list",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternSingleton, SingletonName: "current"},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Operations:  OperationSet{Get: true, List: true},
			},
			wantErr: require.Error,
		},
		{
			name: "list without get",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Operations:  OperationSet{List: true},
			},
			wantErr: require.Error,
		},
		{
			name: "audit emit_on_update without get",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Operations:  OperationSet{Update: true, Create: true},
				Audit:       AuditConfig{EmitOnUpdate: true},
			},
			wantErr: require.Error,
		},
		{
			name: "cache enabled without list",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Cache:       CacheConfig{Enabled: true},
				Operations:  OperationSet{Get: true, Create: true},
			},
			wantErr: require.Error,
		},
		{
			name: "emit_on_get without get operation",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Audit:       AuditConfig{EmitOnGet: true},
			},
			wantErr: require.Error,
		},
		{
			name: "upsert audit without create audit",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Operations:  OperationSet{Get: true, List: true, Create: true, Upsert: true},
				Audit:       AuditConfig{EmitOnUpdate: true, EmitOnCreate: false},
				Cache:       CacheConfig{Enabled: true},
			},
			wantErr: require.Error,
		},
		{
			name: "cache with scoped storage not supported",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternScoped, ScopeBy: "namespace"},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Operations:  OperationSet{Get: true, List: true},
				Cache:       CacheConfig{Enabled: true},
			},
			wantErr: require.Error,
		},
		{
			name: "scoped without cache is valid",
			spec: ResourceSpec{
				ServiceName: "teleport.foo.v1.FooService",
				Kind:        "foo",
				Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternScoped, ScopeBy: "namespace"},
				Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
				Operations:  OperationSet{Get: true, List: true},
			},
			wantErr: require.NoError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.wantErr(t, tc.spec.Validate())
		})
	}
}

func TestPascalToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Webhook", "webhook"},
		{"AccessPolicy", "access_policy"},
		{"AccessMonitoringRule", "access_monitoring_rule"},
		{"Foo", "foo"},
		{"", ""},
		{"A", "a"},
		{"AB", "ab"},
		{"HTMLParser", "html_parser"},
		{"GetURLValue", "get_url_value"},
		{"ALLCAPS", "allcaps"},
		{"IOReader", "io_reader"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			require.Equal(t, tc.want, PascalToSnake(tc.input))
		})
	}
}
