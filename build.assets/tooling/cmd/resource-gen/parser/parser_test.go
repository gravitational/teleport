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
	"testing"

	optionsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/options/v1"
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParseMinimalStandardConfig(t *testing.T) {
	svc := buildServiceDescriptor(t, &optionsv1.ResourceConfig{
		Storage: &optionsv1.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       &optionsv1.StorageConfig_Standard{Standard: &optionsv1.StandardStorage{}},
		},
	}, standardMethods(), standardMessages())

	rs, err := ParseServiceDescriptor(svc)
	require.NoError(t, err)
	require.Equal(t, "teleport.foo.v1.FooService", rs.ServiceName)
	require.Equal(t, "foo", rs.Kind)
	require.Equal(t, spec.StoragePatternStandard, rs.Storage.Pattern)
	require.Equal(t, "foo", rs.Storage.BackendPrefix)
	require.True(t, rs.Operations.Get)
	require.True(t, rs.Operations.List)
	require.True(t, rs.Operations.Create)
	require.True(t, rs.Operations.Update)
	require.False(t, rs.Operations.Upsert)
	require.True(t, rs.Operations.Delete)

	require.True(t, rs.Cache.Enabled)
	require.Equal(t, []string{"metadata.name"}, rs.Cache.Indexes)
	require.Equal(t, "Foo resources", rs.Tctl.Description)
	require.True(t, rs.Tctl.MFARequired)
	require.EqualValues(t, 200, rs.Pagination.DefaultPageSize)
	require.EqualValues(t, 1000, rs.Pagination.MaxPageSize)
}

func TestParseScopedMissingScopeFieldFails(t *testing.T) {
	svc := buildServiceDescriptor(t, &optionsv1.ResourceConfig{
		Storage: &optionsv1.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       &optionsv1.StorageConfig_Scoped{Scoped: &optionsv1.ScopedStorage{By: "username"}},
		},
	}, []*descriptorpb.MethodDescriptorProto{
		method("GetFoo", ".teleport.foo.v1.GetFooRequest", ".teleport.foo.v1.Foo"),
	}, []*descriptorpb.DescriptorProto{
		message("Foo"),
		message("GetFooRequest", strField(1, "name")),
	})

	_, err := ParseServiceDescriptor(svc)
	require.Error(t, err)
	require.ErrorContains(t, err, "username")
}

func TestParseSingletonRejectsList(t *testing.T) {
	svc := buildServiceDescriptor(t, &optionsv1.ResourceConfig{
		Storage: &optionsv1.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       &optionsv1.StorageConfig_Singleton{Singleton: &optionsv1.SingletonStorage{FixedName: "cluster"}},
		},
	}, []*descriptorpb.MethodDescriptorProto{
		method("ListFoos", ".teleport.foo.v1.ListFoosRequest", ".teleport.foo.v1.ListFoosResponse"),
	}, []*descriptorpb.DescriptorProto{
		message("ListFoosRequest", int32Field(1, "page_size"), strField(2, "page_token")),
		message("ListFoosResponse"),
	})

	_, err := ParseServiceDescriptor(svc)
	require.Error(t, err)
	require.ErrorContains(t, err, "singleton storage does not support List")
}

func TestParseSingletonRejectsNameInGetRequest(t *testing.T) {
	svc := buildServiceDescriptor(t, &optionsv1.ResourceConfig{
		Storage: &optionsv1.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       &optionsv1.StorageConfig_Singleton{Singleton: &optionsv1.SingletonStorage{FixedName: "cluster"}},
		},
	}, []*descriptorpb.MethodDescriptorProto{
		method("GetFoo", ".teleport.foo.v1.GetFooRequest", ".teleport.foo.v1.Foo"),
	}, []*descriptorpb.DescriptorProto{
		message("Foo"),
		message("GetFooRequest", strField(1, "name")),
	})

	_, err := ParseServiceDescriptor(svc)
	require.Error(t, err)
	require.ErrorContains(t, err, "singleton Get request must not contain name field")
}

func TestParseSingletonRejectsNameInDeleteRequest(t *testing.T) {
	svc := buildServiceDescriptor(t, &optionsv1.ResourceConfig{
		Storage: &optionsv1.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       &optionsv1.StorageConfig_Singleton{Singleton: &optionsv1.SingletonStorage{FixedName: "cluster"}},
		},
	}, []*descriptorpb.MethodDescriptorProto{
		method("DeleteFoo", ".teleport.foo.v1.DeleteFooRequest", ".teleport.foo.v1.Empty"),
	}, []*descriptorpb.DescriptorProto{
		message("Empty"),
		message("DeleteFooRequest", strField(1, "name")),
	})

	_, err := ParseServiceDescriptor(svc)
	require.Error(t, err)
	require.ErrorContains(t, err, "singleton Delete request must not contain name field")
}

func TestParseMissingResourceConfigFails(t *testing.T) {
	svc := buildServiceDescriptor(t, nil, standardMethods(), standardMessages())

	_, err := ParseServiceDescriptor(svc)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing teleport.resource_config option")
}

func TestParseOverridesDefaults(t *testing.T) {
	svc := buildServiceDescriptor(t, &optionsv1.ResourceConfig{
		Storage: &optionsv1.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       &optionsv1.StorageConfig_Standard{Standard: &optionsv1.StandardStorage{}},
		},
		Cache: &optionsv1.CacheConfig{
			Enabled: boolp(false),
		},
		Tctl: &optionsv1.TctlConfig{
			Description:    "Custom",
			MfaRequired:    boolp(false),
			Columns:        []string{"metadata.name", "spec.value"},
			VerboseColumns: []string{"metadata.name", "metadata.expires"},
		},
		Audit: &optionsv1.AuditConfig{
			EmitOnCreate: boolp(false),
			EmitOnUpdate: boolp(false),
			EmitOnDelete: boolp(false),
		},
		Hooks: &optionsv1.HooksConfig{
			EnableLifecycleHooks: true,
		},
		Pagination: &optionsv1.PaginationConfig{
			DefaultPageSize: int32p(10),
			MaxPageSize:     int32p(20),
		},
	}, standardMethods(), standardMessages())

	rs, err := ParseServiceDescriptor(svc)
	require.NoError(t, err)
	require.False(t, rs.Cache.Enabled)
	require.Equal(t, []string{"metadata.name"}, rs.Cache.Indexes)
	require.Equal(t, "Custom", rs.Tctl.Description)
	require.False(t, rs.Tctl.MFARequired)
	require.Equal(t, []string{"metadata.name", "spec.value"}, rs.Tctl.Columns)
	require.False(t, rs.Audit.EmitOnCreate)
	require.False(t, rs.Audit.EmitOnUpdate)
	require.False(t, rs.Audit.EmitOnDelete)
	require.False(t, rs.Audit.EmitOnGet)
	require.True(t, rs.Hooks.EnableLifecycleHooks)
	require.EqualValues(t, 10, rs.Pagination.DefaultPageSize)
	require.EqualValues(t, 20, rs.Pagination.MaxPageSize)
}

func TestParseEmitOnGetAccepted(t *testing.T) {
	svc := buildServiceDescriptor(t, &optionsv1.ResourceConfig{
		Storage: &optionsv1.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       &optionsv1.StorageConfig_Standard{Standard: &optionsv1.StandardStorage{}},
		},
		Audit: &optionsv1.AuditConfig{
			EmitOnGet: boolp(true),
		},
	}, standardMethods(), standardMessages())

	rs, err := ParseServiceDescriptor(svc)
	require.NoError(t, err)
	require.True(t, rs.Audit.EmitOnGet)
}

func TestParseRejectsCustomCacheIndexes(t *testing.T) {
	svc := buildServiceDescriptor(t, &optionsv1.ResourceConfig{
		Storage: &optionsv1.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       &optionsv1.StorageConfig_Standard{Standard: &optionsv1.StandardStorage{}},
		},
		Cache: &optionsv1.CacheConfig{
			Indexes: []string{"metadata.name", "spec.value"},
		},
	}, standardMethods(), standardMessages())

	_, err := ParseServiceDescriptor(svc)
	require.Error(t, err)
	require.ErrorContains(t, err, "cache.indexes")
}

func standardMethods() []*descriptorpb.MethodDescriptorProto {
	return []*descriptorpb.MethodDescriptorProto{
		method("GetFoo", ".teleport.foo.v1.GetFooRequest", ".teleport.foo.v1.Foo"),
		method("ListFoos", ".teleport.foo.v1.ListFoosRequest", ".teleport.foo.v1.ListFoosResponse"),
		method("CreateFoo", ".teleport.foo.v1.CreateFooRequest", ".teleport.foo.v1.Foo"),
		method("UpdateFoo", ".teleport.foo.v1.UpdateFooRequest", ".teleport.foo.v1.Foo"),
		method("DeleteFoo", ".teleport.foo.v1.DeleteFooRequest", ".teleport.foo.v1.Empty"),
	}
}

func standardMessages() []*descriptorpb.DescriptorProto {
	return []*descriptorpb.DescriptorProto{
		message("Foo"),
		message("Empty"),
		message("ListFoosResponse"),
		message("GetFooRequest", strField(1, "name")),
		message("ListFoosRequest", int32Field(1, "page_size"), strField(2, "page_token")),
		message("CreateFooRequest", msgField(1, "foo", ".teleport.foo.v1.Foo")),
		message("UpdateFooRequest", msgField(1, "foo", ".teleport.foo.v1.Foo")),
		message("DeleteFooRequest", strField(1, "name")),
	}
}

func buildServiceDescriptor(t *testing.T, cfg *optionsv1.ResourceConfig, methods []*descriptorpb.MethodDescriptorProto, messages []*descriptorpb.DescriptorProto) protoreflect.ServiceDescriptor {
	t.Helper()

	fd := &descriptorpb.FileDescriptorProto{
		Name:        strp("test_resource.proto"),
		Package:     strp("teleport.foo.v1"),
		Syntax:      strp("proto3"),
		MessageType: messages,
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name:   strp("FooService"),
			Method: methods,
			Options: func() *descriptorpb.ServiceOptions {
				opts := &descriptorpb.ServiceOptions{}
				if cfg != nil {
					proto.SetExtension(opts, optionsv1.E_ResourceConfig, cfg)
				}
				return opts
			}(),
		}},
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}})
	require.NoError(t, err)

	desc, err := files.FindFileByPath("test_resource.proto")
	require.NoError(t, err)

	svc := desc.Services().ByName("FooService")
	require.NotNil(t, svc)
	return svc
}

func message(name string, fields ...*descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Name:  strp(name),
		Field: fields,
	}
}

func method(name, input, output string) *descriptorpb.MethodDescriptorProto {
	return &descriptorpb.MethodDescriptorProto{
		Name:       strp(name),
		InputType:  strp(input),
		OutputType: strp(output),
	}
}

func strField(number int32, name string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   strp(name),
		Number: int32p(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}
}

func int32Field(number int32, name string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   strp(name),
		Number: int32p(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
	}
}

func msgField(number int32, name, typeName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     strp(name),
		Number:   int32p(number),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		TypeName: strp(typeName),
	}
}

func strp(v string) *string { return &v }
func boolp(v bool) *bool    { return &v }
func int32p(v int32) *int32 { return &v }
