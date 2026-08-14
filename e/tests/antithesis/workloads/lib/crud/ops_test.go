package crud_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/crud"
)

type testResource153 struct {
	kind     string
	subKind  string
	version  string
	metadata *headerv1.Metadata
}

func newTestResource153(kind, name string) *testResource153 {
	return &testResource153{
		kind:    kind,
		version: "v1",
		metadata: &headerv1.Metadata{
			Name: name,
		},
	}
}

func (r *testResource153) GetKind() string {
	return r.kind
}

func (r *testResource153) GetSubKind() string {
	return r.subKind
}

func (r *testResource153) GetVersion() string {
	return r.version
}

func (r *testResource153) GetMetadata() *headerv1.Metadata {
	return r.metadata
}

func (r *testResource153) Clone() types.Resource153 {
	if r == nil {
		return nil
	}
	return &testResource153{
		kind:     r.kind,
		subKind:  r.subKind,
		version:  r.version,
		metadata: proto.Clone(r.metadata).(*headerv1.Metadata),
	}
}

// IsEqual uses the same self-typed signature real Teleport resource types
// use
func (r *testResource153) IsEqual(other *testResource153) bool {
	if r == nil || other == nil {
		return r == other
	}
	return r.kind == other.kind &&
		r.subKind == other.subKind &&
		r.version == other.version &&
		proto.Equal(r.metadata, other.metadata)
}

type otherTestResource153 struct {
	*testResource153
}

type testKindResourceOps struct {
	kind        string
	newResource func(string) (*testResource153, error)
	create      func(context.Context, *testResource153) (*testResource153, error)
	get         func(context.Context, string) (*testResource153, error)
	list        func(context.Context, int, string) ([]*testResource153, string, error)
	update      func(context.Context, *testResource153) (*testResource153, error)
	delete      func(context.Context, string) error
	deleteAll   func(context.Context) error
	equal       func(*testResource153, *testResource153) bool
}

func (o *testKindResourceOps) Kind() string {
	return o.kind
}

func (o *testKindResourceOps) NewResource(name string) (types.Resource153, error) {
	return o.newResource(name)
}

func (o *testKindResourceOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	typed, err := o.cast(resource)
	if err != nil {
		return nil, err
	}
	return o.create(ctx, typed)
}

func (o *testKindResourceOps) Get153(ctx context.Context, name string) (types.Resource153, error) {
	return o.get(ctx, name)
}

func (o *testKindResourceOps) List153(ctx context.Context, pageSize int, pageToken string) ([]types.Resource153, string, error) {
	resources, next, err := o.list(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", err
	}
	out := make([]types.Resource153, 0, len(resources))
	for _, resource := range resources {
		out = append(out, resource)
	}
	return out, next, nil
}

func (o *testKindResourceOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	typed, err := o.cast(resource)
	if err != nil {
		return nil, err
	}
	return o.update(ctx, typed)
}

func (o *testKindResourceOps) Delete(ctx context.Context, name string) error {
	return o.delete(ctx, name)
}

func (o *testKindResourceOps) DeleteAll(ctx context.Context) error {
	if o.deleteAll == nil {
		return trace.NotImplemented("delete-all is not implemented for kind %q", o.kind)
	}
	return o.deleteAll(ctx)
}

func (o *testKindResourceOps) Clone153(resource types.Resource153) types.Resource153 {
	typed, err := o.cast(resource)
	if err != nil {
		panic(err)
	}
	return typed.Clone()
}

func (o *testKindResourceOps) SupportsDeleteAll() bool {
	return o.deleteAll != nil
}

func (o *testKindResourceOps) Equal(a, b types.Resource153) bool {
	ta, err := o.cast(a)
	if err != nil {
		panic(err)
	}
	tb, err := o.cast(b)
	if err != nil {
		panic(err)
	}
	if o.equal == nil {
		panic("resource kind has no Equal comparison configured")
	}
	return o.equal(ta, tb)
}

func (o *testKindResourceOps) cast(resource types.Resource153) (*testResource153, error) {
	typed, ok := resource.(*testResource153)
	if !ok {
		return nil, trace.BadParameter("resource kind %q received unexpected resource type %T", o.kind, resource)
	}
	return typed, nil
}

func TestResourceOpsAdapter(t *testing.T) {
	ctx := t.Context()
	var (
		allCreated []*testResource153
		deleted    bool
		deleteAll  bool
	)

	ops := &testKindResourceOps{
		kind: "header",
		newResource: func(name string) (*testResource153, error) {
			return newTestResource153("header", name), nil
		},
		create: func(_ context.Context, resource *testResource153) (*testResource153, error) {
			allCreated = append(allCreated, resource)
			return resource, nil
		},
		get: func(_ context.Context, name string) (*testResource153, error) {
			require.Len(t, allCreated, 1)
			require.Equal(t, name, allCreated[0].GetMetadata().GetName())
			return allCreated[0], nil
		},
		list: func(_ context.Context, _ int, _ string) ([]*testResource153, string, error) {
			return allCreated, "", nil
		},
		update: func(_ context.Context, resource *testResource153) (*testResource153, error) {
			require.Equal(t, "one", resource.GetMetadata().GetName())
			return resource, nil
		},
		delete: func(_ context.Context, name string) error {
			require.Equal(t, "one", name)
			deleted = true
			return nil
		},
		deleteAll: func(context.Context) error {
			deleteAll = true
			return nil
		},
		equal: (*testResource153).IsEqual,
	}

	resource, err := ops.NewResource("one")
	require.NoError(t, err)
	require.Equal(t, "header", resource.GetKind())

	created, err := ops.Create(ctx, resource)
	require.NoError(t, err)
	require.True(t, ops.Equal(resource, created), "created resource should equal the resource passed to Create")

	got, err := ops.Get153(ctx, "one")
	require.NoError(t, err)
	require.Equal(t, resource, got)

	listed, next, err := ops.List153(ctx, 10, "")
	require.NoError(t, err)
	require.Empty(t, next)
	require.Equal(t, []types.Resource153{resource}, listed)

	updated, err := ops.Update(ctx, resource)
	require.NoError(t, err)
	require.True(t, ops.Equal(resource, updated), "updated resource should equal the resource passed to Update")
	require.NoError(t, ops.Delete(ctx, "one"))
	require.True(t, deleted)

	require.True(t, ops.SupportsDeleteAll())
	require.NoError(t, ops.DeleteAll(ctx))
	require.True(t, deleteAll)

	wrongType := &otherTestResource153{testResource153: newTestResource153("header", "wrong-kind")}
	_, err = ops.Create(ctx, wrongType)
	require.True(t, trace.IsBadParameter(err))
}

func TestAllDefaultOpsAreSuported(t *testing.T) {
	for _, kind := range crud.DefaultKinds() {
		ops, err := crud.OpsForKind(fakeClient{}, kind)
		require.NoError(t, err)
		require.Equal(t, kind, ops.Kind())

		resource, err := ops.NewResource("test-resource")
		require.NoError(t, err)
		require.Equal(t, kind, resource.GetKind())
		require.Equal(t, "test-resource", resource.GetMetadata().GetName())
	}
}

func TestSetStaticLabel(t *testing.T) {
	resource := newTestResource153("header", "one")
	resource.GetMetadata().Labels = map[string]string{"existing": "label"}

	original := resource.GetMetadata().GetLabels()
	require.NoError(t, crud.SetStaticLabel(resource, "updated", "true"))
	require.Equal(t, "label", resource.GetMetadata().GetLabels()["existing"])
	require.Equal(t, "true", resource.GetMetadata().GetLabels()["updated"])

	original["mutated"] = "false"
	require.Empty(t, resource.GetMetadata().GetLabels()["mutated"])
}

func TestLegacyResource153MarshalJSONSyncsMetadata(t *testing.T) {
	resource, err := crud.RoleOps(fakeClient{}).NewResource("role-json")
	require.NoError(t, err)
	require.NoError(t, crud.SetStaticLabel(resource, "scope", "crud-test"))

	data, err := json.Marshal(resource)
	require.NoError(t, err)
	require.Contains(t, string(data), `"name":"role-json"`)
	require.Contains(t, string(data), `"scope":"crud-test"`)
}

func TestResourceOpsEqualPanicsWithoutOverride(t *testing.T) {
	ops := &testKindResourceOps{
		kind: "header",
		newResource: func(name string) (*testResource153, error) {
			return newTestResource153("header", name), nil
		},
	}

	first, err := ops.NewResource("first")
	require.NoError(t, err)
	second, err := ops.NewResource("second")
	require.NoError(t, err)

	require.Panics(t, func() { ops.Equal(first, second) },
		"Equal should panic for a kind with no Equal comparison configured")
	require.Panics(t, func() {
		crud.EqualResourceSlices([]types.Resource153{first}, []types.Resource153{second}, ops.Equal)
	}, "EqualResourceSlices should panic when called with an Equal function that panics")
}

func TestResourceOpsEqualUsesConfiguredComparison(t *testing.T) {
	ops := &testKindResourceOps{
		kind: "header",
		newResource: func(name string) (*testResource153, error) {
			return newTestResource153("header", name), nil
		},
		equal: (*testResource153).IsEqual,
	}

	first, err := ops.NewResource("first")
	require.NoError(t, err)
	firstAgain, err := ops.NewResource("first")
	require.NoError(t, err)
	second, err := ops.NewResource("second")
	require.NoError(t, err)

	require.True(t, ops.Equal(first, firstAgain))
	require.False(t, ops.Equal(first, second))

	require.True(t, crud.EqualResourceSlices(
		[]types.Resource153{first, second},
		[]types.Resource153{second, first},
		ops.Equal,
	), "EqualResourceSlices should ignore ordering")
	require.False(t, crud.EqualResourceSlices(
		[]types.Resource153{first},
		[]types.Resource153{second},
		ops.Equal,
	), "EqualResourceSlices should distinguish different resources")
}

// TestCloneResourceForAllDefaultKinds verifes that every kind returned by [crud.DefaultKinds]
// correctly configures [crud.KindResourceOps.Clone153] and [crud.KindResourceOps.Equal]
func TestCloneResourceForAllDefaultKinds(t *testing.T) {
	for _, kind := range crud.DefaultKinds() {
		t.Run(kind, func(t *testing.T) {
			ops, err := crud.OpsForKind(fakeClient{}, kind)
			require.NoError(t, err)

			original, err := ops.NewResource("clone-test-" + kind)
			require.NoError(t, err)
			require.NotPanics(t, func() {
				cloned := ops.Clone153(original)
				require.NotNil(t, cloned)

				require.True(t, ops.Equal(original, cloned))

				// Mutating labels after cloning must not affect the clone
				require.NoError(t, crud.SetStaticLabel(original, "mutated-after-clone", "true"))
				require.NotContains(t, cloned.GetMetadata().GetLabels(), "mutated-after-clone",
					"mutating original after clone leaked into clone for kind %q", kind)
			})

		})
	}
}
