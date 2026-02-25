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

package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	optionsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/options/v1"
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "missing proto dir",
			args:    []string{"--module=github.com/gravitational/teleport"},
			wantErr: require.Error,
		},
		{
			name:    "missing module",
			args:    []string{"--proto-dir=api/proto"},
			wantErr: require.Error,
		},
		{
			name:    "valid",
			args:    []string{"--proto-dir=api/proto", "--output-dir=.", "--module=github.com/gravitational/teleport", "--dry-run"},
			wantErr: require.NoError,
		},
		{
			name:    "events_only_without_module",
			args:    []string{"--proto-dir=api/proto", "--events-only"},
			wantErr: require.NoError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("resource-gen", flag.ContinueOnError)
			_, err := parseFlags(fs, tc.args)
			tc.wantErr(t, err)
		})
	}
}

func TestRun(t *testing.T) {
	err := runWithWriter(Config{
		ProtoDir:  filepath.Clean("../../../../api/proto"),
		OutputDir: t.TempDir(),
		Module:    "github.com/gravitational/teleport",
		DryRun:    true,
	}, &bytes.Buffer{})
	require.NoError(t, err)
}

func TestRunWithWriterDryRun(t *testing.T) {
	protoDir := writeProtoFixture(t)

	var out bytes.Buffer
	err := runWithWriter(Config{
		ProtoDir:  protoDir,
		OutputDir: t.TempDir(),
		Module:    "github.com/gravitational/teleport",
		DryRun:    true,
	}, &out)
	require.NoError(t, err)
	require.Contains(t, out.String(), "lib/services/foo.gen.go")
	require.Contains(t, out.String(), "lib/services/local/foo.gen.go")
	require.Contains(t, out.String(), "lib/auth/foo/foov1/service.gen.go")
	require.Contains(t, out.String(), "lib/auth/foo_register.gen.go")
	require.Contains(t, out.String(), "lib/services/local/foo_register.gen.go")
	require.Contains(t, out.String(), "lib/cache/foo_register.gen.go")
	require.Contains(t, out.String(), "lib/cache/foo.gen.go")
	require.Contains(t, out.String(), "api/client/foo.gen.go")
	require.Contains(t, out.String(), "tool/tctl/common/resources/foo_register.gen.go")
	require.Contains(t, out.String(), "lib/auth/foo/foov1/service.go (scaffold)")
	require.Contains(t, out.String(), "lib/services/foo.go (scaffold)")
	require.Contains(t, out.String(), "lib/services/foo_test.go (scaffold)")
	require.Contains(t, out.String(), "lib/events/api.gen.go")
	require.Contains(t, out.String(), "lib/events/codes.gen.go")
	require.Contains(t, out.String(), "lib/events/dynamic.gen.go")
	require.Contains(t, out.String(), "lib/events/events_test.gen.go")
	require.Contains(t, out.String(), "api/types/events/oneof.gen.go")
}

func TestRunWithWriterWritesFiles(t *testing.T) {
	protoDir := writeProtoFixture(t)
	outputDir := t.TempDir()

	err := runWithWriter(Config{
		ProtoDir:  protoDir,
		OutputDir: outputDir,
		Module:    "github.com/gravitational/teleport",
		DryRun:    false,
	}, &bytes.Buffer{})
	require.NoError(t, err)

	serviceContent, err := os.ReadFile(filepath.Join(outputDir, "lib", "services", "foo.gen.go"))
	require.NoError(t, err)
	require.Contains(t, string(serviceContent), "type Foos interface")

	backendContent, err := os.ReadFile(filepath.Join(outputDir, "lib", "services", "local", "foo.gen.go"))
	require.NoError(t, err)
	require.Contains(t, string(backendContent), "type FooService struct")
}

func TestResourceOptionsExtensionPresent(t *testing.T) {
	opts := &descriptorpb.ServiceOptions{}
	proto.SetExtension(opts, optionsv1.E_ResourceConfig, &optionsv1.ResourceConfig{})
	require.True(t, proto.HasExtension(opts, optionsv1.E_ResourceConfig))
}

func TestGenerateFilesCacheDisabled(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		KindPascal:  "Foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: false},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
	}

	files, err := generateFiles([]spec.ResourceSpec{rs}, "github.com/gravitational/teleport")
	require.NoError(t, err)

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	require.NotContains(t, paths, filepath.Join("lib", "cache", "foo_register.gen.go"))
	require.NotContains(t, paths, filepath.Join("lib", "cache", "foo.gen.go"))
	// Other files still generated
	require.Contains(t, paths, filepath.Join("lib", "services", "foo.gen.go"))
	require.Contains(t, paths, filepath.Join("lib", "services", "local", "foo.gen.go"))
}

func TestGeneratedFilesHeaderAndNaming(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		KindPascal:  "Foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
	}

	files, err := generateFiles([]spec.ResourceSpec{rs}, "github.com/gravitational/teleport")
	require.NoError(t, err)

	for _, f := range files {
		if f.SkipIfExists {
			// Scaffold files MUST NOT have the generated header.
			require.False(t, strings.HasPrefix(f.Content, generatedHeader),
				"scaffold file %s must not have generated header", f.Path)
			// Scaffold files MUST NOT have .gen.go suffix.
			require.False(t, strings.HasSuffix(f.Path, ".gen.go"),
				"scaffold file %s must not use .gen.go suffix", f.Path)
		} else {
			// Always-overwritten files MUST have the generated header.
			require.True(t, strings.HasPrefix(f.Content, generatedHeader),
				"generated file %s must have generated header", f.Path)
			// Always-overwritten files MUST have .gen.go or .gen_test.go suffix.
			require.True(t, strings.HasSuffix(f.Path, ".gen.go") || strings.HasSuffix(f.Path, ".gen_test.go"),
				"generated file %s must use .gen.go or .gen_test.go suffix", f.Path)
		}
	}
}

func TestGeneratedEventsFilesHeaderAndNaming(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		KindPascal:  "Foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true, Indexes: []string{"metadata.name"}},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "FO"},
	}

	files, err := generateFiles([]spec.ResourceSpec{rs}, "github.com/gravitational/teleport")
	require.NoError(t, err)

	// All 5 cross-resource event files must have header + .gen.go suffix
	eventPaths := []string{
		filepath.Join("lib", "events", "api.gen.go"),
		filepath.Join("lib", "events", "codes.gen.go"),
		filepath.Join("lib", "events", "dynamic.gen.go"),
		filepath.Join("lib", "events", "events_test.gen.go"),
		filepath.Join("api", "types", "events", "oneof.gen.go"),
	}

	for _, ep := range eventPaths {
		found := false
		for _, f := range files {
			if f.Path == ep {
				found = true
				require.True(t, strings.HasPrefix(f.Content, generatedHeader),
					"event file %s must have generated header", ep)
				require.True(t, strings.HasSuffix(f.Path, ".gen.go"),
					"event file %s must have .gen.go suffix", ep)
				require.False(t, f.SkipIfExists,
					"event gathering file %s must not be a scaffold", ep)
				break
			}
		}
		require.True(t, found, "expected event file %s to be generated", ep)
	}

}

func TestValidateCodePrefixRequired(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true, Indexes: []string{"metadata.name"}},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: ""},
	}
	require.Error(t, rs.Validate(), "should fail: emit_on_create is true but code_prefix is empty")
}

func TestValidateCodePrefixFormat(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true, Indexes: []string{"metadata.name"}},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "toolong"},
	}
	require.Error(t, rs.Validate(), "should fail: code_prefix must be 2-4 uppercase ASCII")
}

func TestValidateCodePrefixValid(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true, Indexes: []string{"metadata.name"}},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "FO"},
	}
	require.NoError(t, rs.Validate())
}

func TestGenerateFilesDuplicateCodePrefix(t *testing.T) {
	rs1 := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		KindPascal:  "Foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "FO"},
	}
	rs2 := spec.ResourceSpec{
		ServiceName: "teleport.bar.v1.BarService",
		Kind:        "bar",
		KindPascal:  "Bar",
		Storage:     spec.StorageConfig{BackendPrefix: "bar", Pattern: spec.StoragePatternStandard},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "FO"},
	}
	_, err := generateFiles([]spec.ResourceSpec{rs1, rs2}, "github.com/gravitational/teleport")
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func writeProtoFixture(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "teleport", "options", "v1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "teleport", "foo", "v1"), 0o755))

	optionsProto := `syntax = "proto3";
package teleport.options.v1;
import "google/protobuf/descriptor.proto";
extend google.protobuf.ServiceOptions {
  ResourceConfig resource_config = 50001;
}
message ResourceConfig {
  StorageConfig storage = 1;
  AuditConfig audit = 4;
}
message StorageConfig {
  string backend_prefix = 1;
  oneof pattern {
    StandardStorage standard = 2;
  }
}
message StandardStorage {}
message AuditConfig {
  optional bool emit_on_create = 1;
  optional bool emit_on_update = 2;
  optional bool emit_on_delete = 3;
  optional bool emit_on_get = 4;
  string code_prefix = 5;
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "teleport", "options", "v1", "resource.proto"), []byte(optionsProto), 0o644))

	serviceProto := `syntax = "proto3";
package teleport.foo.v1;
import "teleport/options/v1/resource.proto";
service FooService {
  option (teleport.options.v1.resource_config) = {
    storage: {
      backend_prefix: "foo"
      standard: {}
    }
    audit: {
      emit_on_create: true
      code_prefix: "FO"
    }
  };
  rpc GetFoo(GetFooRequest) returns (Foo);
  rpc ListFoos(ListFoosRequest) returns (ListFoosResponse);
}
message Foo {}
message GetFooRequest { string name = 1; }
message ListFoosRequest { int32 page_size = 1; string page_token = 2; }
message ListFoosResponse { repeated Foo foos = 1; string next_page_token = 2; }
`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "teleport", "foo", "v1", "foo_service.proto"), []byte(serviceProto), 0o644))

	return tmp
}
