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

package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindCandidateProtoFiles(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "a.proto"), []byte(`syntax = "proto3"; package a;`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "b.proto"), []byte(`syntax = "proto3"; package b; // resource_config`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "nested", "c.proto"), []byte(`syntax = "proto3"; package c; // resource_config`), 0o644))

	files, err := findCandidateProtoFiles(tmp)
	require.NoError(t, err)
	require.Equal(t, []string{"b.proto", "nested/c.proto"}, files)
}

func TestParseProtoDir(t *testing.T) {
	protoDir := writeProtoFixture(t)

	specs, err := ParseProtoDir(context.Background(), protoDir)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	require.Equal(t, "teleport.foo.v1.FooService", specs[0].ServiceName)
	require.Equal(t, "foo", specs[0].Kind)
	require.Equal(t, "foo", specs[0].Storage.BackendPrefix)
	require.True(t, specs[0].Operations.Get)
}

func TestParseProtoDirNoResources(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "simple.proto"), []byte(`syntax = "proto3"; package foo.v1;`), 0o644))

	specs, err := ParseProtoDir(context.Background(), tmp)
	require.NoError(t, err)
	require.Empty(t, specs)
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
}
message StorageConfig {
  string backend_prefix = 1;
  oneof pattern {
    StandardStorage standard = 2;
  }
}
message StandardStorage {}
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
