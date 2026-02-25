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

package generators

import (
	"strings"
	"testing"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/stretchr/testify/require"
)

const testModule = "github.com/gravitational/teleport"

func testSpec(ops spec.OperationSet) spec.ResourceSpec {
	return spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		KindPascal:  "Foo",
		Storage: spec.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       spec.StoragePatternStandard,
		},
		Pagination: spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "FO"},
		Operations: ops,
	}
}

func testSingletonSpec(ops spec.OperationSet) spec.ResourceSpec {
	return spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		KindPascal:  "Foo",
		Storage: spec.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       spec.StoragePatternSingleton,
			SingletonName: "current",
		},
		Pagination: spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnDelete: true, CodePrefix: "FO"},
		Operations: ops,
	}
}

func testScopedSpec(ops spec.OperationSet) spec.ResourceSpec {
	return spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		KindPascal:  "Foo",
		Storage: spec.StorageConfig{
			BackendPrefix: "foo",
			Pattern:       spec.StoragePatternScoped,
			ScopeBy:       "username",
		},
		Pagination: spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "FO"},
		Operations: ops,
	}
}

func TestGenerateServiceInterfaceStandardCRUD(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateServiceInterface(rs, testModule)
	require.NoError(t, err)

	// Getter sub-interface with read-only methods.
	require.Contains(t, got, "type FoosGetter interface")
	require.Contains(t, got, "GetFoo(ctx context.Context, name string) (*foov1.Foo, error)")
	require.Contains(t, got, "ListFoos(ctx context.Context, pageSize int64, pageToken string) ([]*foov1.Foo, string, error)")

	// Full interface embeds getter and adds write methods.
	require.Contains(t, got, "type Foos interface")
	require.Contains(t, got, "\tFoosGetter\n")
	require.Contains(t, got, "CreateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error)")
	require.Contains(t, got, "UpdateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error)")
	require.Contains(t, got, "DeleteFoo(ctx context.Context, name string) error")

	require.Contains(t, got, `foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"`)
	require.Contains(t, got, "func MarshalFoo(object *foov1.Foo, opts ...MarshalOption) ([]byte, error)")
	require.Contains(t, got, "func UnmarshalFoo(data []byte, opts ...MarshalOption) (*foov1.Foo, error)")
	require.NotContains(t, got, "ValidateFoo")
}

func TestGenerateServiceInterfaceSingleton(t *testing.T) {
	rs := testSingletonSpec(spec.OperationSet{Get: true, Create: true, Delete: true})

	got, err := GenerateServiceInterface(rs, testModule)
	require.NoError(t, err)
	// Singleton Get omits name parameter
	require.Contains(t, got, "GetFoo(ctx context.Context) (*foov1.Foo, error)")
	require.NotContains(t, got, "GetFoo(ctx context.Context, name string)")
	// Singleton Delete omits name parameter
	require.Contains(t, got, "DeleteFoo(ctx context.Context) error")
	require.NotContains(t, got, "DeleteFoo(ctx context.Context, name string)")
	// No List for singleton
	require.NotContains(t, got, "ListFoos")
}

func TestGenerateServiceInterfaceScopedGet(t *testing.T) {
	rs := testScopedSpec(spec.OperationSet{Get: true, List: true})

	got, err := GenerateServiceInterface(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "GetFoo(ctx context.Context, username string, name string) (*foov1.Foo, error)")
	require.Contains(t, got, "ListFoos(ctx context.Context, username string, pageSize int64, pageToken string)")
}

func TestGenerateBackendImplementation(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateBackendImplementation(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "generic.ServiceWrapper[*foov1.Foo]")
	require.Contains(t, got, "generic.NewServiceWrapper(generic.ServiceConfig[*foov1.Foo]")
	require.Contains(t, got, "services.MarshalFoo")
	require.Contains(t, got, "services.UnmarshalFoo")
	require.Contains(t, got, "func (s *FooService) GetFoo")
	require.Contains(t, got, "func (s *FooService) ListFoos")
	require.Contains(t, got, "func (s *FooService) CreateFoo")
	require.Contains(t, got, "s.service.ConditionalUpdateResource")
	require.Contains(t, got, "func (s *FooService) DeleteFoo")
}

func TestGenerateBackendImplementationSingleton(t *testing.T) {
	rs := testSingletonSpec(spec.OperationSet{Get: true, Delete: true})

	got, err := GenerateBackendImplementation(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, `singletonName = "current"`)
	// Singleton Get doesn't take a name parameter
	require.NotContains(t, got, "func (s *FooService) GetFoo(ctx context.Context, name string)")
	require.Contains(t, got, "func (s *FooService) GetFoo(ctx context.Context)")
	require.Contains(t, got, "s.service.GetResource(ctx, singletonName)")
	// Singleton Delete doesn't take a name parameter
	require.NotContains(t, got, "func (s *FooService) DeleteFoo(ctx context.Context, name string)")
	require.Contains(t, got, "func (s *FooService) DeleteFoo(ctx context.Context)")
	require.Contains(t, got, "s.service.DeleteResource(ctx, singletonName)")
}

func TestGenerateBackendImplementationPagination(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true})
	rs.Pagination = spec.PaginationConfig{DefaultPageSize: 100, MaxPageSize: 500}

	got, err := GenerateBackendImplementation(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "fooDefaultPageSize = 100")
	require.Contains(t, got, "fooMaxPageSize     = 500")
	require.Contains(t, got, "if pageSize <= 0 {")
	require.Contains(t, got, "pageSize = fooDefaultPageSize")
	require.Contains(t, got, "if pageSize > fooMaxPageSize {")
	require.Contains(t, got, "pageSize = fooMaxPageSize")
}

func TestGenerateGRPCService(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package foov1")
	require.Contains(t, got, `foov1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"`)
	require.Contains(t, got, "type Reader interface")
	require.Contains(t, got, "GetFoo(ctx context.Context, name string)")
	require.Contains(t, got, "ListFoos(ctx context.Context, pageSize int64, pageToken string)")
	require.Contains(t, got, "func (s *Service) GetFoo")
	require.Contains(t, got, "func (s *Service) ListFoos")
	require.Contains(t, got, "func (s *Service) CreateFoo")
	require.Contains(t, got, "func (s *Service) UpdateFoo")
	require.Contains(t, got, "func (s *Service) DeleteFoo")
	require.Contains(t, got, "emptypb.Empty")
	require.Contains(t, got, "services.ValidateFoo(req.GetFoo())")
	require.Contains(t, got, "s.emitCreateAuditEvent(ctx, rsp, authCtx, err)")
	require.Contains(t, got, "oldFoo, err := s.reader.GetFoo(ctx, req.GetFoo().GetMetadata().GetName())")
	require.Contains(t, got, "s.emitUpdateAuditEvent(ctx, oldFoo, req.GetFoo(), authCtx, err)")
	require.Contains(t, got, "deleteName := req.GetName()")
	require.Contains(t, got, "err = s.backend.DeleteFoo(ctx, deleteName)")
	require.Contains(t, got, "s.emitDeleteAuditEvent(ctx, deleteName, authCtx, err)")
	require.Contains(t, got, "func eventStatus(err error) apievents.Status")
	require.Contains(t, got, "func getExpires(ts *timestamppb.Timestamp) time.Time")
	require.NotContains(t, got, "TODO(resource-gen)")

	// Auth delegation calls
	require.Contains(t, got, "s.authorize(ctx, types.VerbRead)")
	require.Contains(t, got, "s.authorizeMutation(ctx, types.VerbCreate)")
	require.Contains(t, got, "s.authorizeMutation(ctx, types.VerbUpdate)")
	require.Contains(t, got, "s.authorizeMutation(ctx, types.VerbDelete)")

	// Structs and inline auth moved to scaffold
	require.NotContains(t, got, "type ServiceConfig struct")
	require.NotContains(t, got, "type Service struct")
	require.NotContains(t, got, "func NewService(")
	require.NotContains(t, got, "CheckAndSetDefaults")
	require.NotContains(t, got, "s.authorizer.Authorize")
	require.NotContains(t, got, "CheckAccessToKind")
	require.NotContains(t, got, "AuthorizeAdminActionAllowReusedMFA")
}

func TestGenerateAPIClient(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateAPIClient(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, `foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"`)
	require.Contains(t, got, "func (c *Client) GetFoo")
	require.Contains(t, got, "func (c *Client) ListFoos")
	require.Contains(t, got, "func (c *Client) CreateFoo")
	require.Contains(t, got, "func (c *Client) UpdateFoo")
	require.Contains(t, got, "func (c *Client) DeleteFoo")
	require.Contains(t, got, "foov1.NewFooServiceClient(c.conn)")
}

func TestGenerateAPIClientSingleton(t *testing.T) {
	rs := testSingletonSpec(spec.OperationSet{Get: true, Create: true, Delete: true})

	got, err := GenerateAPIClient(rs, testModule)
	require.NoError(t, err)
	// Singleton Get has no name param
	require.Contains(t, got, "func (c *Client) GetFoo(ctx context.Context) (*foov1.Foo, error)")
	require.NotContains(t, got, "Name: name")
	// Singleton Delete has no name param
	require.Contains(t, got, "func (c *Client) DeleteFoo(ctx context.Context) error")
	// Create still passes the resource
	require.Contains(t, got, "func (c *Client) CreateFoo")
}

func TestServicePathParts(t *testing.T) {
	rs := spec.ResourceSpec{ServiceName: "teleport.foo.v2.FooService", Kind: "foo"}
	resource, pkgDir := ServicePathParts(rs)
	require.Equal(t, "foo", resource)
	require.Equal(t, "foov2", pkgDir)
}

func TestProtoGoImportPath(t *testing.T) {
	got := protoGoImportPath("teleport.widget.v1.WidgetService", "github.com/gravitational/teleport")
	require.Equal(t, "github.com/gravitational/teleport/api/gen/proto/go/teleport/widget/v1", got)
}

func TestProtoPackageAlias(t *testing.T) {
	require.Equal(t, "widgetv1", protoPackageAlias("teleport.widget.v1.WidgetService"))
	require.Equal(t, "foov2", protoPackageAlias("teleport.foo.v2.FooService"))
}

func TestPluralize(t *testing.T) {
	require.Equal(t, "Widgets", pluralize("Widget"))
	require.Equal(t, "Foos", pluralize("Foo"))
	require.Equal(t, "Policies", pluralize("Policy"))
	require.Equal(t, "Accesses", pluralize("Access"))
}

func TestGenerateAuthRegistration(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Cache.Enabled = true

	got, err := GenerateAuthRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package auth")
	require.NotContains(t, got, "ACTION REQUIRED")
	require.NotContains(t, got, "var _ = func(s Services)")
	require.Contains(t, got, "RegisterGeneratedGRPCService")
	require.Contains(t, got, `foov1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"`)
	require.Contains(t, got, `foov1 "github.com/gravitational/teleport/lib/auth/foo/foov1"`)
	require.Contains(t, got, "foov1.NewService(foov1.ServiceConfig{")
	require.Contains(t, got, "cfg.AuthServer.Services")
	require.Contains(t, got, "cfg.AuthServer.Cache")
	require.Contains(t, got, "cfg.Emitter")
	require.Contains(t, got, "foov1pb.RegisterFooServiceServer(server, service)")
}

func TestGenerateAuthRegistrationNoCache(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	// Cache.Enabled defaults to false.

	got, err := GenerateAuthRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "cfg.AuthServer.Services")
	require.NotContains(t, got, "cfg.AuthServer.Cache")
	require.Contains(t, got, "reads go directly to backend")
}

func TestGenerateLocalParserRegistration(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})

	got, err := GenerateLocalParserRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package local")
	require.Contains(t, got, "RegisterGeneratedResourceParser")
	require.Contains(t, got, "type fooParser struct")
	require.Contains(t, got, "baseParser")
	require.Contains(t, got, "backend.NewKey(fooPrefix)")
	require.Contains(t, got, "types.OpDelete")
	require.Contains(t, got, "types.OpPut")
	require.Contains(t, got, "services.UnmarshalFoo")
	require.Contains(t, got, "types.Resource153ToLegacy(r)")
	require.Contains(t, got, "apidefaults.Namespace")
	require.Contains(t, got, "types.KindFoo")
}

func TestGenerateCacheRegistration(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})

	got, err := GenerateCacheRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package cache")
	require.NotContains(t, got, "ACTION REQUIRED")
	require.Contains(t, got, "GENERATED CHECK")
	require.Contains(t, got, "add \"Foos services.Foos\" to the Config struct in lib/cache/cache.go")
	require.Contains(t, got, "var _ services.Foos = Config{}.Foos")
	require.Contains(t, got, "RegisterGeneratedCollectionBuilder")
	require.Contains(t, got, `foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"`)
	require.Contains(t, got, "proto.CloneOf[*foov1.Foo]")
	require.Contains(t, got, "clientutils.Resources")
	require.Contains(t, got, "stream.Collect")
	require.Contains(t, got, "config.Foos.ListFoos")
	require.Contains(t, got, "headerTransform")
	require.Contains(t, got, "headerv1.Metadata{Name: hdr.Metadata.Name}")
	require.Contains(t, got, "types.KindFoo")
	require.NotContains(t, got, `Kind: "foo"`)
}

func TestGenerateCacheAccessors(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})

	got, err := GenerateCacheAccessors(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package cache")
	require.Contains(t, got, `foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"`)
	require.Contains(t, got, "func (c *Cache) fooCollection() *collection[*foov1.Foo, fooIndex]")
	require.Contains(t, got, `resourceKind{kind: types.KindFoo}`)
	require.Contains(t, got, "func (c *Cache) GetFoo(ctx context.Context, name string)")
	require.Contains(t, got, "genericGetter[*foov1.Foo, fooIndex]")
	require.Contains(t, got, "fooNameIndex")
	require.Contains(t, got, "c.Config.Foos.GetFoo")
	require.Contains(t, got, "func (c *Cache) ListFoos(ctx context.Context, pageSize int64, pageToken string)")
	require.Contains(t, got, "genericLister[*foov1.Foo, fooIndex]")
	require.Contains(t, got, "c.Config.Foos.ListFoos")
	require.Contains(t, got, "t.GetMetadata().GetName()")
	require.Contains(t, got, `c.Tracer.Start(ctx, "cache/GetFoo")`)
	require.Contains(t, got, `c.Tracer.Start(ctx, "cache/ListFoos")`)
}

func TestGenerateCacheAccessorsGetOnly(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true})

	got, err := GenerateCacheAccessors(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "func (c *Cache) GetFoo(ctx context.Context, name string)")
	// Both Get and List accessors present since both ops are enabled
	require.Contains(t, got, "func (c *Cache) ListFoos")
	require.NotContains(t, got, "CreateFoo")
}

func TestGenerateTCTLRegistration(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateTCTLRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package resources")
	require.Contains(t, got, "RegisterGeneratedHandler")
	require.Contains(t, got, `foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"`)
	require.Contains(t, got, "type fooCollection struct")
	require.Contains(t, got, "types.Resource153ToLegacy")
	require.Contains(t, got, "CUSTOMIZE")
	require.Contains(t, got, "common.FormatLabels")
	require.Contains(t, got, "asciitable.MakeTable")
	require.Contains(t, got, `kind:          types.KindFoo`)
	require.Contains(t, got, "getHandler:    getFoo")
	require.Contains(t, got, "createHandler: createFoo")
	require.Contains(t, got, "updateHandler: updateFoo")
	require.Contains(t, got, "deleteHandler: deleteFoo")
	require.Contains(t, got, "func getFoo(")
	require.Contains(t, got, "client.GetFoo(ctx, ref.Name)")
	require.Contains(t, got, "client.ListFoos")
	require.Contains(t, got, "services.UnmarshalFoo(raw.Raw")
	require.Contains(t, got, "client.CreateFoo")
	require.Contains(t, got, "client.UpdateFoo")
	require.Contains(t, got, "client.DeleteFoo")
}

func TestGenerateTCTLRegistrationWithUpsert(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true, Upsert: true})

	got, err := GenerateTCTLRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "opts.Force")
	require.Contains(t, got, "client.UpsertFoo")
}

func TestGenerateGRPCServiceSingleton(t *testing.T) {
	rs := testSingletonSpec(spec.OperationSet{Get: true, Create: true, Delete: true})

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	// Singleton Reader interface has no name param
	require.Contains(t, got, "GetFoo(ctx context.Context) (*foov1pb.Foo, error)")
	require.NotContains(t, got, "GetFoo(ctx context.Context, name string)")
	// Singleton Get calls reader without name
	require.Contains(t, got, "s.reader.GetFoo(ctx)")
	require.NotContains(t, got, "req.GetName()")
	// Singleton Delete uses fixed name
	require.Contains(t, got, `deleteName := "current"`)
	require.Contains(t, got, "s.backend.DeleteFoo(ctx)")
	require.NotContains(t, got, "s.backend.DeleteFoo(ctx, deleteName)")
}

func TestGenerateGRPCServiceWithHooks(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, Create: true, Update: true, Delete: true})
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: true}

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)

	// Hooks struct defined
	require.Contains(t, got, "type Hooks struct")
	require.Contains(t, got, "BeforeCreate func(context.Context,")
	require.Contains(t, got, "AfterCreate  func(context.Context,")
	require.Contains(t, got, "BeforeUpdate func(context.Context,")
	require.Contains(t, got, "AfterUpdate  func(context.Context,")
	require.Contains(t, got, "BeforeDelete func(context.Context, string) error")
	require.Contains(t, got, "AfterDelete  func(context.Context, string)")

	// Hook callsites
	require.Contains(t, got, "s.hooks != nil && s.hooks.BeforeCreate != nil")
	require.Contains(t, got, "s.hooks.BeforeCreate(ctx,")
	require.Contains(t, got, "s.hooks != nil && s.hooks.AfterCreate != nil")
	require.Contains(t, got, "s.hooks.AfterCreate(ctx,")

	// Config/Service structs moved to scaffold
	require.NotContains(t, got, "type ServiceConfig struct")
}

func TestGenerateGRPCServiceWithoutHooks(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, Create: true, Update: true, Delete: true})
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: false}

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.NotContains(t, got, "type Hooks struct")
	require.NotContains(t, got, "BeforeCreate")
	require.NotContains(t, got, "AfterCreate")
}

func TestGenerateGRPCServiceCustom(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package foov1")

	// Struct definitions moved from gen template
	require.Contains(t, got, "type ServiceConfig struct")
	require.Contains(t, got, "Authorizer authz.Authorizer")
	require.Contains(t, got, "Backend    services.Foos")
	require.Contains(t, got, "Reader     Reader")
	require.Contains(t, got, "Emitter    apievents.Emitter")
	require.Contains(t, got, "CheckAndSetDefaults")
	require.Contains(t, got, "type Service struct")
	require.Contains(t, got, "foov1pb.UnimplementedFooServiceServer")
	require.Contains(t, got, "func NewService(cfg ServiceConfig)")
	require.Contains(t, got, "// TODO: add resource-specific dependencies")
	require.Contains(t, got, "// TODO: add resource-specific fields")

	// Auth methods
	require.Contains(t, got, "func (s *Service) authorize(")
	require.Contains(t, got, "func (s *Service) authorizeMutation(")
	require.Contains(t, got, "s.authorizer.Authorize(ctx)")
	require.Contains(t, got, "authCtx.CheckAccessToKind(types.KindFoo, verb, verbs...)")
	require.Contains(t, got, "authCtx.AuthorizeAdminActionAllowReusedMFA()")

	// Correct signatures
	require.Contains(t, got, "func (s *Service) emitCreateAuditEvent(ctx context.Context, foo *foov1pb.Foo, authCtx *authz.Context, err error)")
	require.Contains(t, got, "func (s *Service) emitUpdateAuditEvent(ctx context.Context, old, new *foov1pb.Foo, authCtx *authz.Context, err error)")
	require.Contains(t, got, "func (s *Service) emitDeleteAuditEvent(ctx context.Context, name string, authCtx *authz.Context, err error)")

	// Derived event type names
	require.Contains(t, got, "&apievents.FooCreate{")
	require.Contains(t, got, "libevents.FooCreateEvent")
	require.Contains(t, got, "libevents.FooCreateCode")
	require.Contains(t, got, "&apievents.FooUpdate{")
	require.Contains(t, got, "&apievents.FooDelete{")

	// Generic boilerplate
	require.Contains(t, got, "authCtx.GetUserMetadata()")
	require.Contains(t, got, "authz.ConnectionMetadata(ctx)")
	require.Contains(t, got, "eventStatus(err)")
	require.Contains(t, got, "getExpires(")
	require.Contains(t, got, "slog.WarnContext(ctx,")

	// FIXME markers for resource-specific fields
	require.Contains(t, got, "// FIXME: add resource-specific event fields here.")

	// No upsert (not enabled in this test)
	require.NotContains(t, got, "emitUpsertAuditEvent")
}

func TestGenerateGRPCServiceCustomWithUpsert(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Upsert: true, Delete: true})

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)

	// Upsert delegates to create or update
	require.Contains(t, got, "func (s *Service) emitUpsertAuditEvent(")
	require.Contains(t, got, "if old == nil {")
	require.Contains(t, got, "s.emitCreateAuditEvent(ctx, new, authCtx, err)")
	require.Contains(t, got, "s.emitUpdateAuditEvent(ctx, old, new, authCtx, err)")
}

func TestGenerateGRPCServiceAuditDisabled(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	rs.Audit = spec.AuditConfig{
		EmitOnCreate: false,
		EmitOnUpdate: true,
		EmitOnDelete: false,
		CodePrefix:   "FO",
	}

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.NotContains(t, got, "emitCreateAuditEvent")
	require.Contains(t, got, "emitUpdateAuditEvent")
	require.NotContains(t, got, "emitDeleteAuditEvent")
}

func TestGenerateGRPCServiceCustomAuditDisabled(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	rs.Audit = spec.AuditConfig{
		EmitOnCreate: false,
		EmitOnUpdate: true,
		EmitOnDelete: false,
		CodePrefix:   "FO",
	}

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)
	require.NotContains(t, got, "emitCreateAuditEvent")
	require.Contains(t, got, "emitUpdateAuditEvent")
	require.NotContains(t, got, "emitDeleteAuditEvent")
}

func TestGenerateGRPCServiceAllAuditDisabled(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	rs.Audit = spec.AuditConfig{
		EmitOnCreate: false,
		EmitOnUpdate: false,
		EmitOnDelete: false,
	}

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.NotContains(t, got, "eventStatus")
	require.NotContains(t, got, "getExpires")
	require.NotContains(t, got, "emitCreateAuditEvent")
	require.NotContains(t, got, "emitUpdateAuditEvent")
	require.NotContains(t, got, "emitDeleteAuditEvent")
	require.NotContains(t, got, "emitGetAuditEvent")
}

func TestGenerateGRPCServiceUpsertAudit(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Upsert: true, Delete: true})

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "!trace.IsNotFound(err)")
	require.Contains(t, got, "s.emitUpsertAuditEvent(ctx, oldFoo, req.GetFoo(), authCtx, err)")
}

func TestGenerateGRPCServiceCustomWithHooks(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, Create: true, Update: true, Delete: true})
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: true}

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "Hooks      *Hooks")
	require.Contains(t, got, "hooks      *Hooks")
	require.Contains(t, got, "hooks:      cfg.Hooks,")
}

func TestGenerateGRPCServiceCustomWithHooksAllOps(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Upsert: true, Delete: true})
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: true}

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)

	// ServiceConfig has Hooks field
	require.Contains(t, got, "Hooks      *Hooks")
	// Service struct has hooks field
	require.Contains(t, got, "hooks      *Hooks")
	// Constructor wires hooks
	require.Contains(t, got, "hooks:      cfg.Hooks,")
}

func TestGenerateGRPCServiceHooksMatchOps(t *testing.T) {
	// Only Create and Delete — no Update/Upsert hooks
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Delete: true})
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: true}

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)

	// Create hooks present
	require.Contains(t, got, "BeforeCreate func(context.Context,")
	require.Contains(t, got, "AfterCreate  func(context.Context,")
	// Delete hooks present
	require.Contains(t, got, "BeforeDelete func(context.Context, string) error")
	require.Contains(t, got, "AfterDelete  func(context.Context, string)")
	// Update/Upsert hooks absent (ops not enabled)
	require.NotContains(t, got, "BeforeUpdate")
	require.NotContains(t, got, "AfterUpdate")
	require.NotContains(t, got, "BeforeUpsert")
	require.NotContains(t, got, "AfterUpsert")
}

func TestGenerateGRPCServiceCustomAllAuditDisabled(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	rs.Audit = spec.AuditConfig{
		EmitOnCreate: false,
		EmitOnUpdate: false,
		EmitOnDelete: false,
	}

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package foov1")
	require.NotContains(t, got, "emitCreateAuditEvent")
	require.NotContains(t, got, "emitUpdateAuditEvent")
	require.NotContains(t, got, "emitDeleteAuditEvent")
	require.NotContains(t, got, "emitGetAuditEvent")

	// Struct defs and auth methods always present even without audit
	require.Contains(t, got, "type ServiceConfig struct")
	require.Contains(t, got, "type Service struct")
	require.Contains(t, got, "func NewService(cfg ServiceConfig)")
	require.Contains(t, got, "func (s *Service) authorize(")
	require.Contains(t, got, "func (s *Service) authorizeMutation(")
}

func TestResolveColumns(t *testing.T) {
	cols := resolveColumns([]string{"metadata.name", "spec.flavor", "metadata.expires"}, nil)
	require.Len(t, cols, 3)
	require.Equal(t, "Name", cols[0].Header)
	require.Equal(t, "r.GetMetadata().GetName()", cols[0].Getter)
	require.False(t, cols[0].IsTimestamp)
	require.Equal(t, "Flavor", cols[1].Header)
	require.Equal(t, "r.GetSpec().GetFlavor()", cols[1].Getter)
	require.False(t, cols[1].IsTimestamp)
	require.Equal(t, "Expires", cols[2].Header)
	require.Equal(t, "r.GetMetadata().GetExpires()", cols[2].Getter)
	require.True(t, cols[2].IsTimestamp)
}

func TestResolveColumnsSnakeCase(t *testing.T) {
	cols := resolveColumns([]string{"spec.power_level", "spec.display_name_override"}, nil)
	require.Len(t, cols, 2)
	// snake_case → CamelCase getter (matches protobuf Go codegen)
	require.Equal(t, "Power Level", cols[0].Header)
	require.Equal(t, "r.GetSpec().GetPowerLevel()", cols[0].Getter)
	require.False(t, cols[0].IsTimestamp)
	// Multi-underscore
	require.Equal(t, "Display Name Override", cols[1].Header)
	require.Equal(t, "r.GetSpec().GetDisplayNameOverride()", cols[1].Getter)
	require.False(t, cols[1].IsTimestamp)
}

func TestSnakeToCamel(t *testing.T) {
	require.Equal(t, "PowerLevel", snakeToCamel("power_level"))
	require.Equal(t, "Name", snakeToCamel("name"))
	require.Equal(t, "DisplayNameOverride", snakeToCamel("display_name_override"))
	require.Equal(t, "Metadata", snakeToCamel("metadata"))
}

func TestSnakeToTitle(t *testing.T) {
	require.Equal(t, "Power Level", snakeToTitle("power_level"))
	require.Equal(t, "Name", snakeToTitle("name"))
	require.Equal(t, "Display Name Override", snakeToTitle("display_name_override"))
}

func TestGenerateTCTLRegistrationColumns(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	rs.Tctl.Columns = []string{"metadata.name", "spec.flavor"}
	rs.Tctl.VerboseColumns = []string{"metadata.name", "spec.flavor", "metadata.expires"}

	got, err := GenerateTCTLRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, `"Name"`)
	require.Contains(t, got, `"Flavor"`)
	require.Contains(t, got, `r.GetMetadata().GetName()`)
	require.Contains(t, got, `r.GetSpec().GetFlavor()`)
	// Verbose adds expires
	require.Contains(t, got, `"Expires"`)
	require.Contains(t, got, `formatFooTimestamp(r.GetMetadata().GetExpires())`)
	// Should not have the CUSTOMIZE comment when columns are configured
	require.NotContains(t, got, "CUSTOMIZE")
	// Should have resource-prefixed timestamp helper (no duplicate symbol risk)
	require.Contains(t, got, "func formatFooTimestamp(")
	require.NotContains(t, got, "func formatTimestamp(")
	require.Contains(t, got, "time.RFC3339")
	// Non-timestamp columns wrapped with fmt.Sprintf for type safety
	require.Contains(t, got, `fmt.Sprintf("%v", r.GetMetadata().GetName())`)
	// No trailing commas before closing brace
	require.NotContains(t, got, ", }")
}

func TestGenerateTCTLRegistrationSingleton(t *testing.T) {
	rs := testSingletonSpec(spec.OperationSet{Get: true, Create: true, Delete: true})

	got, err := GenerateTCTLRegistration(rs, testModule)
	require.NoError(t, err)
	// Singleton get calls client.GetFoo directly (no List fallback)
	require.Contains(t, got, "client.GetFoo(ctx)")
	require.NotContains(t, got, "client.ListFoos")
	require.NotContains(t, got, "ref.Name")
	// Singleton delete calls client.DeleteFoo without name
	require.Contains(t, got, "client.DeleteFoo(ctx)")
	// No update handler (Update not in ops)
	require.NotContains(t, got, "updateHandler")
	require.NotContains(t, got, "updateFoo")
	require.NotContains(t, got, "client.UpdateFoo")
	// Singleton flag set
	require.Contains(t, got, "singleton:     true")
	// No unused imports
	require.NotContains(t, got, "clientutils")
	require.NotContains(t, got, "stream")
}

func TestGenerateTCTLRegistrationOpGating(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, Create: true})
	// Only Get and Create — no Update, Delete, or List

	got, err := GenerateTCTLRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "getHandler:    getFoo")
	require.Contains(t, got, "createHandler: createFoo")
	require.NotContains(t, got, "updateHandler")
	require.NotContains(t, got, "deleteHandler")
	// No List, so get requires name
	require.Contains(t, got, `trace.BadParameter("resource name is required")`)
}

func TestGenerateTCTLRegistrationDefaultColumns(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	// Default: tctl.columns and verbose are empty in testSpec, so falls back to CUSTOMIZE path

	got, err := GenerateTCTLRegistration(rs, testModule)
	require.NoError(t, err)
	// Default falls back to CUSTOMIZE path (no configured columns)
	require.Contains(t, got, "CUSTOMIZE")
}

func TestGenerateValidation(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateValidation(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package services")
	require.Contains(t, got, "func ValidateFoo(")
	require.Contains(t, got, "trace.NotImplemented")
	require.Contains(t, got, `foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"`)
}

func TestGenerateValidationTest(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateValidationTest(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package services")
	require.Contains(t, got, "func TestValidateFoo(t *testing.T)")
	require.Contains(t, got, "nil resource")
	require.Contains(t, got, "nil metadata")
	require.Contains(t, got, "empty name")
	require.Contains(t, got, "nil spec")
	require.Contains(t, got, "valid minimal")
	require.Contains(t, got, "ValidateFoo(tt.input)")
	require.Contains(t, got, "headerv1.Metadata")
	require.Contains(t, got, "FooSpec{}")
}

func TestGenerateGRPCServiceCustomSingleton(t *testing.T) {
	rs := testSingletonSpec(spec.OperationSet{Get: true, Create: true, Delete: true})

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "type ServiceConfig struct")
	require.Contains(t, got, "type Service struct")
	require.Contains(t, got, "emitCreateAuditEvent")
	require.Contains(t, got, "emitDeleteAuditEvent")
	// No update audit (singleton spec has EmitOnUpdate: false by default)
	require.NotContains(t, got, "emitUpdateAuditEvent")
}

func TestGenerateTCTLRegistrationColumnsOnly(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	rs.Tctl.Columns = []string{"metadata.name", "spec.flavor"}
	rs.Tctl.VerboseColumns = nil // Columns set, VerboseColumns empty

	got, err := GenerateTCTLRegistration(rs, testModule)
	require.NoError(t, err)
	// Should use simple headers (no if verbose branch)
	require.Contains(t, got, `"Name"`)
	require.Contains(t, got, `"Flavor"`)
	require.NotContains(t, got, "CUSTOMIZE")
	// No verbose branch when VerboseColumns is nil
	require.NotContains(t, got, "if verbose")
}

func TestGenerateServiceInterfaceAllOpsFalse(t *testing.T) {
	rs := testSpec(spec.OperationSet{})

	got, err := GenerateServiceInterface(rs, testModule)
	require.NoError(t, err)
	// Interface exists but has no methods
	require.Contains(t, got, "type Foos interface")
	require.NotContains(t, got, "GetFoo")
	require.NotContains(t, got, "ListFoos")
	require.NotContains(t, got, "CreateFoo")
}

func TestNewResourceBase(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	base := newResourceBase(rs, testModule)
	require.Equal(t, "Foo", base.Kind)
	require.Equal(t, "foo", base.Lower)
	require.Equal(t, "Foos", base.Plural)
	require.Equal(t, "foov1", base.PkgAlias)
	require.Equal(t, "*foov1.Foo", base.QualType)
	require.False(t, base.IsSingleton)
}

func TestNewResourceBaseSingleton(t *testing.T) {
	rs := testSingletonSpec(spec.OperationSet{Get: true, Create: true, Delete: true})
	base := newResourceBase(rs, testModule)
	require.True(t, base.IsSingleton)
	require.Equal(t, "current", base.SingletonName)
}

func TestGenerateBackendImplementationScoped(t *testing.T) {
	rs := testScopedSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateBackendImplementation(rs, testModule)
	require.NoError(t, err)
	// Scoped Get takes scope + name and uses WithPrefix
	require.Contains(t, got, "func (s *FooService) GetFoo(ctx context.Context, username string, name string)")
	require.Contains(t, got, "s.service.WithPrefix(username).GetResource(ctx, name)")
	// Scoped List takes scope param and uses WithPrefix
	require.Contains(t, got, "func (s *FooService) ListFoos(ctx context.Context, username string, pageSize int64, pageToken string)")
	require.Contains(t, got, "s.service.WithPrefix(username).ListResources(ctx, int(pageSize), pageToken)")
	// Scoped Create extracts scope from resource via GetSpec()
	require.Contains(t, got, "func (s *FooService) CreateFoo")
	require.Contains(t, got, "s.service.WithPrefix(foo.GetSpec().GetUsername()).CreateResource(ctx, foo)")
	// Scoped Update extracts scope from resource via GetSpec()
	require.Contains(t, got, "s.service.WithPrefix(foo.GetSpec().GetUsername()).ConditionalUpdateResource(ctx, foo)")
	// Scoped Delete takes scope + name and uses WithPrefix
	require.Contains(t, got, "func (s *FooService) DeleteFoo(ctx context.Context, username string, name string)")
	require.Contains(t, got, "s.service.WithPrefix(username).DeleteResource(ctx, name)")
}

func TestGenerateGRPCServiceScoped(t *testing.T) {
	rs := testScopedSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	// Scoped Reader interface has scope param
	require.Contains(t, got, "GetFoo(ctx context.Context, username string, name string)")
	require.Contains(t, got, "ListFoos(ctx context.Context, username string, pageSize int64, pageToken string)")
	// Scoped Get uses req.GetUsername()
	require.Contains(t, got, "func (s *Service) GetFoo")
	require.Contains(t, got, "s.reader.GetFoo(ctx, req.GetUsername(), req.GetName())")
	// Scoped List passes scope from request
	require.Contains(t, got, "func (s *Service) ListFoos")
	require.Contains(t, got, "s.reader.ListFoos(ctx, req.GetUsername(), int64(req.GetPageSize()), req.GetPageToken())")
	// Scoped Delete uses req.GetUsername()
	require.Contains(t, got, "func (s *Service) DeleteFoo")
	require.Contains(t, got, "s.backend.DeleteFoo(ctx, req.GetUsername(), deleteName)")
	// Scoped Update audit pre-fetch uses scope from resource
	require.Contains(t, got, "s.reader.GetFoo(ctx, req.GetFoo().GetSpec().GetUsername(), req.GetFoo().GetMetadata().GetName())")
	// Create/Update unchanged (scope embedded in resource)
	require.Contains(t, got, "func (s *Service) CreateFoo")
}

func TestGenerateAPIClientScoped(t *testing.T) {
	rs := testScopedSpec(spec.OperationSet{Get: true, List: true, Create: true, Delete: true})

	got, err := GenerateAPIClient(rs, testModule)
	require.NoError(t, err)
	// Scoped Get takes scope + name and passes in request
	require.Contains(t, got, "func (c *Client) GetFoo(ctx context.Context, username string, name string)")
	require.Contains(t, got, "Username: username,")
	require.Contains(t, got, "Name: name,")
	// Scoped List takes scope param and passes in request
	require.Contains(t, got, "func (c *Client) ListFoos(ctx context.Context, username string, pageSize int64, pageToken string)")
	require.Contains(t, got, "Username: username,")
	// Scoped Delete takes scope + name
	require.Contains(t, got, "func (c *Client) DeleteFoo(ctx context.Context, username string, name string)")
	// Create unchanged (scope embedded in resource)
	require.Contains(t, got, "func (c *Client) CreateFoo(ctx context.Context, foo *foov1.Foo)")
}

func TestGenerateKindConstants(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true}),
	}
	// Override kind for variety
	specs[0].Kind = "widget"
	specs[0].KindPascal = "Widget"
	specs[0].ServiceName = "teleport.widget.v1.WidgetService"

	got, err := GenerateKindConstants(specs)
	require.NoError(t, err)
	require.Contains(t, got, "package types")
	require.Contains(t, got, `KindWidget = "widget"`)
}

func TestGenerateKindConstantsMultiple(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true}),
	}
	spec2 := testSpec(spec.OperationSet{Get: true})
	spec2.Kind = "gadget"
	spec2.KindPascal = "Gadget"
	spec2.ServiceName = "teleport.gadget.v1.GadgetService"
	specs = append(specs, spec2)

	got, err := GenerateKindConstants(specs)
	require.NoError(t, err)
	require.Contains(t, got, `KindFoo = "foo"`)
	require.Contains(t, got, `KindGadget = "gadget"`)
}

func TestGenerateServicesGathering(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true, Create: true}),
	}

	got, err := GenerateServicesGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package auth")
	require.Contains(t, got, "type servicesGenerated struct")
	require.Contains(t, got, "services.Foos")
	require.Contains(t, got, "func newServicesGenerated(cfg *InitConfig)")
	require.Contains(t, got, "gen.Foos, err = local.NewFooService(cfg.Backend)")
	require.Contains(t, got, "trace.Wrap(err)")
}

func TestGenerateServicesGatheringMultiple(t *testing.T) {
	// Input in reverse alphabetical order to verify sorting.
	spec1 := testSpec(spec.OperationSet{Get: true, Create: true})
	spec1.Kind = "gadget"
	spec1.KindPascal = "Gadget"
	spec1.ServiceName = "teleport.gadget.v1.GadgetService"
	spec2 := testSpec(spec.OperationSet{Get: true, List: true})
	specs := []spec.ResourceSpec{spec1, spec2}

	got, err := GenerateServicesGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "services.Foos")
	require.Contains(t, got, "services.Gadgets")
	require.Contains(t, got, "gen.Foos, err = local.NewFooService(cfg.Backend)")
	require.Contains(t, got, "gen.Gadgets, err = local.NewGadgetService(cfg.Backend)")
	require.Less(t, strings.Index(got, "Foos"), strings.Index(got, "Gadgets"))
}

func TestGenerateCacheGathering(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true, Create: true}),
	}
	specs[0].Cache.Enabled = true

	got, err := GenerateCacheGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package cache")
	require.Contains(t, got, "type GeneratedConfig struct")
	require.Contains(t, got, "services.Foos")
}

func TestGenerateCacheGatheringExcludesDisabledCache(t *testing.T) {
	spec1 := testSpec(spec.OperationSet{Get: true, List: true})
	spec1.Cache.Enabled = true
	spec2 := testSpec(spec.OperationSet{Get: true})
	spec2.Kind = "gadget"
	spec2.KindPascal = "Gadget"
	spec2.ServiceName = "teleport.gadget.v1.GadgetService"
	spec2.Cache.Enabled = false
	specs := []spec.ResourceSpec{spec1, spec2}

	got, err := GenerateCacheGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "services.Foos")
	require.NotContains(t, got, "Gadgets")
}

func TestGenerateCacheGatheringMultiple(t *testing.T) {
	// Input in reverse alphabetical order to verify sorting.
	spec1 := testSpec(spec.OperationSet{Get: true, List: true})
	spec1.Kind = "gadget"
	spec1.KindPascal = "Gadget"
	spec1.ServiceName = "teleport.gadget.v1.GadgetService"
	spec1.Cache.Enabled = true
	spec2 := testSpec(spec.OperationSet{Get: true, List: true})
	spec2.Cache.Enabled = true
	specs := []spec.ResourceSpec{spec1, spec2}

	got, err := GenerateCacheGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "services.Foos")
	require.Contains(t, got, "services.Gadgets")
	require.Less(t, strings.Index(got, "Foos"), strings.Index(got, "Gadgets"))
}

func TestGenerateCacheGatheringAllDisabled(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true}),
	}
	// Cache.Enabled defaults to false

	got, err := GenerateCacheGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "type GeneratedConfig struct")
	require.NotContains(t, got, "services.")
}

func TestGenerateAuthclientGathering(t *testing.T) {
	s := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	s.Cache.Enabled = true
	specs := []spec.ResourceSpec{s}

	got, err := GenerateAuthclientGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package authclient")
	require.Contains(t, got, "type cacheGeneratedServices interface")
	require.Contains(t, got, "services.FoosGetter")
}

func TestGenerateAuthclientGatheringMultiple(t *testing.T) {
	// Input in reverse alphabetical order to verify sorting.
	spec1 := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	spec1.Kind = "gadget"
	spec1.KindPascal = "Gadget"
	spec1.ServiceName = "teleport.gadget.v1.GadgetService"
	spec1.Cache.Enabled = true
	spec2 := testSpec(spec.OperationSet{Get: true, List: true})
	spec2.Cache.Enabled = true
	specs := []spec.ResourceSpec{spec1, spec2}

	got, err := GenerateAuthclientGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "services.FoosGetter")
	require.Contains(t, got, "services.GadgetsGetter")
	require.Less(t, strings.Index(got, "Foos"), strings.Index(got, "Gadgets"))
}

func TestGenerateGRPCServiceWithEmitOnGet(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Audit.EmitOnGet = true

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "s.emitGetAuditEvent(ctx,")
	// Get handler must capture authCtx when emitting
	require.Contains(t, got, "authCtx, err := s.authorize(ctx, types.VerbRead)")
}

func TestGenerateGRPCServiceWithoutEmitOnGet(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Audit.EmitOnGet = false

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.NotContains(t, got, "emitGetAuditEvent")
	// Without emit, Get handler discards authCtx
	require.Contains(t, got, "_, err := s.authorize(ctx, types.VerbRead)")
}

func TestGenerateGRPCServiceSingletonWithEmitOnGet(t *testing.T) {
	rs := testSingletonSpec(spec.OperationSet{Get: true, Create: true, Delete: true})
	rs.Audit.EmitOnGet = true

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, `s.emitGetAuditEvent(ctx, "current", authCtx, err)`)
	require.Contains(t, got, "authCtx, err := s.authorize(ctx, types.VerbRead)")
}

func TestGenerateEventsAPI(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "cookie",
			KindPascal: "Cookie",
			Operations: spec.OperationSet{Create: true, Update: true, Delete: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "CK"},
		},
	}
	got, err := GenerateEventsAPI(specs)
	require.NoError(t, err)
	require.Contains(t, got, `CookieCreateEvent = "resource.cookie.create"`)
	require.Contains(t, got, `CookieUpdateEvent = "resource.cookie.update"`)
	require.Contains(t, got, `CookieDeleteEvent = "resource.cookie.delete"`)
	require.NotContains(t, got, "GetEvent") // emit_on_get is false
}

func TestGenerateEventsCodes(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "cookie",
			KindPascal: "Cookie",
			Operations: spec.OperationSet{Create: true, Update: true, Delete: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "CK"},
		},
	}
	got, err := GenerateEventsCodes(specs)
	require.NoError(t, err)
	require.Contains(t, got, `CookieCreateCode = "CK001I"`)
	require.Contains(t, got, `CookieUpdateCode = "CK002I"`)
	require.Contains(t, got, `CookieDeleteCode = "CK003I"`)
}

func TestGenerateEventsDynamic(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:        "cookie",
			KindPascal:  "Cookie",
			ServiceName: "teleport.cookie.v1.CookieService",
			Operations:  spec.OperationSet{Create: true, Delete: true},
			Audit:       spec.AuditConfig{EmitOnCreate: true, EmitOnDelete: true, CodePrefix: "CK"},
		},
	}
	got, err := GenerateEventsDynamic(specs, "github.com/gravitational/teleport")
	require.NoError(t, err)
	require.Contains(t, got, "CookieCreateEvent")
	require.Contains(t, got, "apievents.CookieCreate{}")
	require.Contains(t, got, "init()")
}

func TestGenerateEventsTest(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:        "cookie",
			KindPascal:  "Cookie",
			ServiceName: "teleport.cookie.v1.CookieService",
			Operations:  spec.OperationSet{Create: true},
			Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "CK"},
		},
	}
	got, err := GenerateEventsTest(specs, "github.com/gravitational/teleport")
	require.NoError(t, err)
	require.Contains(t, got, "CookieCreateEvent")
	require.Contains(t, got, "CookieCreateCode")
	require.Contains(t, got, "apievents.CookieCreate{}")
	require.Contains(t, got, "init()")
}

func TestGenerateEventsOneOf(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:        "cookie",
			KindPascal:  "Cookie",
			ServiceName: "teleport.cookie.v1.CookieService",
			Operations:  spec.OperationSet{Create: true, Delete: true},
			Audit:       spec.AuditConfig{EmitOnCreate: true, EmitOnDelete: true, CodePrefix: "CK"},
		},
	}
	got, err := GenerateEventsOneOf(specs)
	require.NoError(t, err)
	require.Contains(t, got, "*CookieCreate")
	require.Contains(t, got, "*CookieDelete")
	require.Contains(t, got, "init()")
}

func TestGenerateEventsProtoScaffold(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	got, err := GenerateEventsProtoScaffold(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "message FooCreate")
	require.Contains(t, got, "message FooUpdate")
	require.Contains(t, got, "message FooDelete")
	require.NotContains(t, got, "message FooGet") // emit_on_get is false by default in testSpec
	require.Contains(t, got, "events.Metadata")
	require.Contains(t, got, "events.ResourceMetadata")
}

func TestGenerateGRPCServiceCustomWithEmitOnGet(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Audit.EmitOnGet = true

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "func (s *Service) emitGetAuditEvent(")
	require.Contains(t, got, "&apievents.FooGet{")
	require.Contains(t, got, "libevents.FooGetEvent")
	require.Contains(t, got, "libevents.FooGetCode")
}

func TestGenerateServiceInterfaceScopedDelete(t *testing.T) {
	rs := testScopedSpec(spec.OperationSet{Get: true, List: true, Delete: true})

	got, err := GenerateServiceInterface(rs, testModule)
	require.NoError(t, err)
	// Scoped Delete has scope + name params
	require.Contains(t, got, "DeleteFoo(ctx context.Context, username string, name string) error")
	require.NotContains(t, got, "DeleteFoo(ctx context.Context, name string) error")
}


func TestNewResourceBaseScoped(t *testing.T) {
	rs := testScopedSpec(spec.OperationSet{Get: true, List: true})
	base := newResourceBase(rs, testModule)
	require.True(t, base.IsScoped)
	require.Equal(t, "username", base.ScopeBy)
	require.Equal(t, "Username", base.ScopeByPascal)
	require.False(t, base.IsSingleton)
}

func TestGenerateBackendImplementationScopedUpsert(t *testing.T) {
	rs := testScopedSpec(spec.OperationSet{Get: true, List: true, Create: true, Upsert: true})

	got, err := GenerateBackendImplementation(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "s.service.WithPrefix(foo.GetSpec().GetUsername()).UpsertResource(ctx, foo)")
}

func TestResolveColumnsTimestampSet(t *testing.T) {
	tsSet := map[string]bool{"status.last_delivery": true}
	cols := resolveColumns([]string{"metadata.name", "status.last_delivery"}, tsSet)
	require.Len(t, cols, 2)
	require.False(t, cols[0].IsTimestamp)
	require.True(t, cols[1].IsTimestamp)
	require.Equal(t, "Last Delivery", cols[1].Header)
	require.Equal(t, "r.GetStatus().GetLastDelivery()", cols[1].Getter)
}

func TestGenerateAuthRegistrationWithHooks(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Cache.Enabled = true
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: true}

	got, err := GenerateAuthRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "foov1.DefaultHooks()")
}

func TestGenerateAuthRegistrationWithoutHooks(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Cache.Enabled = true
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: false}

	got, err := GenerateAuthRegistration(rs, testModule)
	require.NoError(t, err)
	require.NotContains(t, got, "DefaultHooks")
}

func TestGenerateTCTLRegistrationTimestampColumns(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true})
	rs.Tctl.Columns = []string{"metadata.name", "status.last_delivery"}
	rs.Tctl.TimestampColumns = []string{"status.last_delivery"}

	got, err := GenerateTCTLRegistration(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, `formatFooTimestamp(r.GetStatus().GetLastDelivery())`)
	require.Contains(t, got, "func formatFooTimestamp(")
}

func TestGenerateShortcutGathering(t *testing.T) {
	specs := []spec.ResourceSpec{
		{Kind: "webhook", KindPascal: "Webhook"},
	}
	got, err := GenerateShortcutGathering(specs)
	require.NoError(t, err)
	require.Contains(t, got, `"webhook": "webhook"`)
	require.Contains(t, got, `"webhooks": "webhook"`)
	require.Contains(t, got, "generatedShortcuts")
}

func TestGenerateShortcutGatheringMultiple(t *testing.T) {
	specs := []spec.ResourceSpec{
		{Kind: "webhook", KindPascal: "Webhook"},
		{Kind: "access_policy", KindPascal: "AccessPolicy"},
	}
	got, err := GenerateShortcutGathering(specs)
	require.NoError(t, err)
	require.Contains(t, got, `"access_policy": "access_policy"`)
	require.Contains(t, got, `"access_policies": "access_policy"`)
	require.Contains(t, got, `"webhook": "webhook"`)
	require.Contains(t, got, `"webhooks": "webhook"`)
}

func TestGenerateShortcutGatheringSorted(t *testing.T) {
	specs := []spec.ResourceSpec{
		{Kind: "zebra", KindPascal: "Zebra"},
		{Kind: "alpha", KindPascal: "Alpha"},
	}
	got, err := GenerateShortcutGathering(specs)
	require.NoError(t, err)
	// Entries should be sorted by alias.
	alphaIdx := strings.Index(got, `"alpha"`)
	zebraIdx := strings.Index(got, `"zebra"`)
	require.Greater(t, zebraIdx, alphaIdx)
}
