// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package generic

import (
	"fmt"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// testUnscopedPrefix is the backend prefix the test wrapper uses for unscoped
	// resources.
	testUnscopedPrefix = "generic_prefix"
	// testScopedTopPrefix is the top-level backend prefix under which the test
	// wrapper namespaces scoped resources.
	testScopedTopPrefix = "scoped"
)

type scopedTestResource153 struct {
	Metadata *headerv1.Metadata
	Scope    string
	Spec     scopedTestResource153Spec
}

type scopedTestResource153Spec struct {
	Data string
}

func (t *scopedTestResource153) GetMetadata() *headerv1.Metadata {
	return t.Metadata
}

func (t *scopedTestResource153) GetScope() string {
	return t.Scope
}

func specDataFor(sqn scopes.QualifiedName) string {
	return fmt.Sprintf("spec-data(scope=%q,name=%q)", sqn.Scope, sqn.Name)
}

func newScopedTestResource153(sqn scopes.QualifiedName) *scopedTestResource153 {
	tr := &scopedTestResource153{
		Metadata: headerv1.Metadata_builder{Name: sqn.Name}.Build(),
		Scope:    sqn.Scope,
		Spec:     scopedTestResource153Spec{Data: specDataFor(sqn)},
	}
	tr.Metadata.SetExpires(timestamppb.New(time.Now().AddDate(0, 0, 3)))
	return tr
}

func marshalScopedResource153(resource *scopedTestResource153, opts ...services.MarshalOption) ([]byte, error) {
	// TODO(strideynet): It feels a little janky utilizing fast marshal rather
	// than Proto marshal (which is what 99.9% of RFD153 resources are going to
	// use). Could we add a "foo" resource to `/api` for testing?
	return utils.FastMarshal(resource)
}

func unmarshalScopedResource153(data []byte, opts ...services.MarshalOption) (*scopedTestResource153, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var r scopedTestResource153
	if err := utils.FastUnmarshal(data, &r); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if r.Metadata == nil {
		r.Metadata = &headerv1.Metadata{}
	}
	if cfg.Revision != "" {
		r.Metadata.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		r.Metadata.SetExpires(timestamppb.New(cfg.Expires))
	}
	return &r, nil
}

func newScopeAwareWrapperForTest(t *testing.T) *ScopeAwareServiceWrapper[*scopedTestResource153] {
	t.Helper()

	memBackend, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewScopeAwareServiceWrapper(ScopeAwareServiceWrapperConfig[*scopedTestResource153]{
		Backend:               memBackend,
		ResourceKind:          "scoped_generic_resource",
		UnscopedBackendPrefix: backend.NewKey(testUnscopedPrefix),
		ScopedBackendPrefix:   backend.NewKey(testScopedTopPrefix, testUnscopedPrefix),
		PageLimit:             200,
		MarshalFunc:           marshalScopedResource153,
		UnmarshalFunc:         unmarshalScopedResource153,
	})
	require.NoError(t, err)
	return service
}

func names(resources []*scopedTestResource153) []scopes.QualifiedName {
	out := make([]scopes.QualifiedName, 0, len(resources))
	for _, r := range resources {
		out = append(out, scopes.QualifiedName{Scope: r.GetScope(), Name: r.GetMetadata().GetName()})
	}
	return out
}

func collectStream(t *testing.T, seq iter.Seq2[*scopedTestResource153, error]) []*scopedTestResource153 {
	t.Helper()
	var out []*scopedTestResource153
	for r, err := range seq {
		require.NoError(t, err)
		out = append(out, r)
	}
	return out
}

// requireResourceBody asserts the resource read back carries exactly the body we
// stored for the given scope-qualified name. Because the spec data is never
// encoded in the backend key, its correct round-trip proves the whole resource
// body — including its scope — is sourced from the persisted value, and not
// reconstructed from the storage key.
func requireResourceBody(t *testing.T, want scopes.QualifiedName, got *scopedTestResource153) {
	t.Helper()
	require.Equal(t, want.Name, got.GetMetadata().GetName())
	require.Equal(t, want.Scope, got.GetScope())
	require.Equal(t, specDataFor(want), got.Spec.Data)
}

// TestScopeAwareServiceWrapper_E2E is an end-to-end smoke test that
// exercises the key behaviors in order.
func TestScopeAwareServiceWrapper_E2E(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)
	ctx := t.Context()

	unscoped := scopes.QualifiedName{Name: "foo"}
	scoped := scopes.QualifiedName{Scope: "/security", Name: "foo"}

	// A scoped and an unscoped resource may share a name; they live in distinct
	// key ranges.
	_, err := svc.CreateResource(ctx, newScopedTestResource153(unscoped))
	require.NoError(t, err)
	_, err = svc.CreateResource(ctx, newScopedTestResource153(scoped))
	require.NoError(t, err)

	// Each resolves only under its own scope-qualified name, returning the body
	// stored for that scope.
	got, err := svc.GetResource(ctx, unscoped)
	require.NoError(t, err)
	requireResourceBody(t, unscoped, got)

	got, err = svc.GetResource(ctx, scoped)
	require.NoError(t, err)
	requireResourceBody(t, scoped, got)

	// An unscoped name does not resolve under a scope, and a scoped name does
	// not resolve as unscoped.
	_, err = svc.GetResource(ctx, scopes.QualifiedName{Scope: "/other", Name: "foo"})
	require.True(t, trace.IsNotFound(err), "expected not found, got %v", err)

	// Deleting one leaves the other intact.
	require.NoError(t, svc.DeleteResource(ctx, unscoped))
	_, err = svc.GetResource(ctx, unscoped)
	require.True(t, trace.IsNotFound(err), "expected not found, got %v", err)
	_, err = svc.GetResource(ctx, scoped)
	require.NoError(t, err)
}

func TestScopeAwareServiceWrapper_CreateResource(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)
	ctx := t.Context()

	t.Run("creates unscoped and scoped resources", func(t *testing.T) {
		for _, qn := range []scopes.QualifiedName{
			{Name: "foo"},
			{Scope: "/security", Name: "foo"},
		} {
			created, err := svc.CreateResource(ctx, newScopedTestResource153(qn))
			require.NoError(t, err)
			require.NotEmpty(t, created.GetMetadata().GetRevision(), "create must populate a revision")

			got, err := svc.GetResource(ctx, qn)
			require.NoError(t, err)
			requireResourceBody(t, qn, got)
		}
	})

	t.Run("rejects duplicate within the same scope", func(t *testing.T) {
		qn := scopes.QualifiedName{Scope: "/eng", Name: "dup"}
		_, err := svc.CreateResource(ctx, newScopedTestResource153(qn))
		require.NoError(t, err)
		_, err = svc.CreateResource(ctx, newScopedTestResource153(qn))
		require.True(t, trace.IsAlreadyExists(err), "expected AlreadyExists, got %v", err)
	})

	t.Run("same name in different scopes does not conflict", func(t *testing.T) {
		a := scopes.QualifiedName{Scope: "/a", Name: "shared"}
		b := scopes.QualifiedName{Scope: "/b", Name: "shared"}
		_, err := svc.CreateResource(ctx, newScopedTestResource153(a))
		require.NoError(t, err)
		_, err = svc.CreateResource(ctx, newScopedTestResource153(b))
		require.NoError(t, err)

		gotA, err := svc.GetResource(ctx, a)
		require.NoError(t, err)
		requireResourceBody(t, a, gotA)
		gotB, err := svc.GetResource(ctx, b)
		require.NoError(t, err)
		requireResourceBody(t, b, gotB)
	})
}

func TestScopeAwareServiceWrapper_UpsertResource(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)
	ctx := t.Context()

	for _, tc := range []struct {
		name string
		qn   scopes.QualifiedName
	}{
		{"unscoped", scopes.QualifiedName{Name: "up"}},
		{"scoped", scopes.QualifiedName{Scope: "/security", Name: "up"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Upsert creates when absent.
			first := newScopedTestResource153(tc.qn)
			first.Spec.Data = "first"
			_, err := svc.UpsertResource(ctx, first)
			require.NoError(t, err)
			got, err := svc.GetResource(ctx, tc.qn)
			require.NoError(t, err)
			require.Equal(t, "first", got.Spec.Data)

			// Upsert overwrites when present, with no revision and no AlreadyExists.
			second := newScopedTestResource153(tc.qn)
			second.Spec.Data = "second"
			_, err = svc.UpsertResource(ctx, second)
			require.NoError(t, err)
			got, err = svc.GetResource(ctx, tc.qn)
			require.NoError(t, err)
			require.Equal(t, "second", got.Spec.Data)
		})
	}

	t.Run("routes by scope", func(t *testing.T) {
		// Upserting a scoped resource must not clobber an unscoped one of the
		// same name.
		unscoped := scopes.QualifiedName{Name: "router"}
		scoped := scopes.QualifiedName{Scope: "/eng", Name: "router"}

		u := newScopedTestResource153(unscoped)
		u.Spec.Data = "unscoped"
		_, err := svc.UpsertResource(ctx, u)
		require.NoError(t, err)

		s := newScopedTestResource153(scoped)
		s.Spec.Data = "scoped"
		_, err = svc.UpsertResource(ctx, s)
		require.NoError(t, err)

		gotU, err := svc.GetResource(ctx, unscoped)
		require.NoError(t, err)
		require.Equal(t, "unscoped", gotU.Spec.Data)
		gotS, err := svc.GetResource(ctx, scoped)
		require.NoError(t, err)
		require.Equal(t, "scoped", gotS.Spec.Data)
	})
}

func TestScopeAwareServiceWrapper_ConditionalUpdateResource(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)
	ctx := t.Context()

	for _, tc := range []struct {
		name string
		qn   scopes.QualifiedName
	}{
		{"unscoped", scopes.QualifiedName{Name: "cu"}},
		{"scoped", scopes.QualifiedName{Scope: "/security", Name: "cu"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created, err := svc.CreateResource(ctx, newScopedTestResource153(tc.qn))
			require.NoError(t, err)
			revBefore := created.GetMetadata().GetRevision()

			// A matching revision updates the body in place and rotates the revision.
			created.Spec.Data = "updated"
			updated, err := svc.ConditionalUpdateResource(ctx, created)
			require.NoError(t, err)
			require.Equal(t, "updated", updated.Spec.Data)
			require.NotEqual(t, revBefore, updated.GetMetadata().GetRevision(), "update must rotate the revision")

			got, err := svc.GetResource(ctx, tc.qn)
			require.NoError(t, err)
			require.Equal(t, "updated", got.Spec.Data)
			require.Equal(t, tc.qn.Scope, got.GetScope())

			// A stale revision is rejected and leaves the stored resource untouched.
			updated.GetMetadata().SetRevision("not-the-right-revision")
			updated.Spec.Data = "should-not-persist"
			_, err = svc.ConditionalUpdateResource(ctx, updated)
			require.True(t, trace.IsCompareFailed(err), "expected CompareFailed, got %v", err)

			got, err = svc.GetResource(ctx, tc.qn)
			require.NoError(t, err)
			require.Equal(t, "updated", got.Spec.Data)
		})
	}

	t.Run("missing resource errors", func(t *testing.T) {
		_, err := svc.ConditionalUpdateResource(ctx, newScopedTestResource153(scopes.QualifiedName{Name: "ghost"}))
		require.Error(t, err)
	})
}

func TestScopeAwareServiceWrapper_GetResource(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)
	ctx := t.Context()

	unscoped := scopes.QualifiedName{Name: "foo"}
	scoped := scopes.QualifiedName{Scope: "/security/eu", Name: "foo"}
	scopedOnly := scopes.QualifiedName{Scope: "/security", Name: "sec-only"}
	for _, qn := range []scopes.QualifiedName{unscoped, scoped, scopedOnly} {
		_, err := svc.CreateResource(ctx, newScopedTestResource153(qn))
		require.NoError(t, err)
	}

	t.Run("unscoped", func(t *testing.T) {
		got, err := svc.GetResource(ctx, unscoped)
		require.NoError(t, err)
		requireResourceBody(t, unscoped, got)
	})

	t.Run("scoped", func(t *testing.T) {
		got, err := svc.GetResource(ctx, scoped)
		require.NoError(t, err)
		requireResourceBody(t, scoped, got)
	})

	t.Run("wrong scope is not found", func(t *testing.T) {
		_, err := svc.GetResource(ctx, scopes.QualifiedName{Scope: "/other", Name: "foo"})
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	})

	t.Run("scoped resource does not leak into the unscoped range", func(t *testing.T) {
		_, err := svc.GetResource(ctx, scopes.QualifiedName{Name: "sec-only"})
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	})
}

func TestScopeAwareServiceWrapper_DeleteResource(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)
	ctx := t.Context()

	t.Run("deletes only the addressed scope", func(t *testing.T) {
		unscoped := scopes.QualifiedName{Name: "foo"}
		scoped := scopes.QualifiedName{Scope: "/security", Name: "foo"}
		for _, qn := range []scopes.QualifiedName{unscoped, scoped} {
			_, err := svc.CreateResource(ctx, newScopedTestResource153(qn))
			require.NoError(t, err)
		}

		// Deleting the unscoped resource leaves the scoped one intact.
		require.NoError(t, svc.DeleteResource(ctx, unscoped))
		_, err := svc.GetResource(ctx, unscoped)
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
		got, err := svc.GetResource(ctx, scoped)
		require.NoError(t, err)
		requireResourceBody(t, scoped, got)

		// And deleting the scoped resource removes it too.
		require.NoError(t, svc.DeleteResource(ctx, scoped))
		_, err = svc.GetResource(ctx, scoped)
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	})

	t.Run("wrong scope does not delete", func(t *testing.T) {
		qn := scopes.QualifiedName{Scope: "/eng", Name: "keep"}
		_, err := svc.CreateResource(ctx, newScopedTestResource153(qn))
		require.NoError(t, err)

		// Deleting under a different scope must not remove it, and is NotFound.
		err = svc.DeleteResource(ctx, scopes.QualifiedName{Scope: "/other", Name: "keep"})
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
		got, err := svc.GetResource(ctx, qn)
		require.NoError(t, err)
		requireResourceBody(t, qn, got)
	})

	t.Run("missing is not found", func(t *testing.T) {
		err := svc.DeleteResource(ctx, scopes.QualifiedName{Name: "ghost"})
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	})
}

func TestScopeAwareServiceWrapper_DeleteAllResources(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)
	ctx := t.Context()

	for _, qn := range []scopes.QualifiedName{
		{Name: "a"},
		{Name: "b"},
		{Scope: "/security", Name: "x"},
		{Scope: "/security/eu", Name: "y"},
	} {
		_, err := svc.CreateResource(ctx, newScopedTestResource153(qn))
		require.NoError(t, err)
	}

	require.NoError(t, svc.DeleteAllResources(ctx))

	// Both the scoped and unscoped ranges are emptied.
	page, next, err := svc.ListResources(ctx, 100, "")
	require.NoError(t, err)
	require.Empty(t, next)
	require.Empty(t, page)
}

func TestScopeAwareServiceWrapper_ListResourcesWithFilter(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)
	ctx := t.Context()

	created := []scopes.QualifiedName{
		{Name: "keep-a"},
		{Name: "drop-a"},
		{Scope: "/security", Name: "keep-b"},
		{Scope: "/security", Name: "drop-b"},
		{Scope: "/security/eu", Name: "keep-c"},
	}
	for _, qn := range created {
		_, err := svc.CreateResource(ctx, newScopedTestResource153(qn))
		require.NoError(t, err)
	}

	keep := func(r *scopedTestResource153) bool {
		return strings.HasPrefix(r.GetMetadata().GetName(), "keep-")
	}
	// The matcher must apply across both the unscoped and scoped ranges, with
	// unscoped matches ordered first.
	want := []scopes.QualifiedName{
		{Name: "keep-a"},
		{Scope: "/security", Name: "keep-b"},
		{Scope: "/security/eu", Name: "keep-c"},
	}

	t.Run("single page", func(t *testing.T) {
		got, next, err := svc.ListResourcesWithFilter(ctx, 100, "", keep)
		require.NoError(t, err)
		require.Empty(t, next)
		require.Equal(t, want, names(got))
	})

	t.Run("paginated preserves filter and order across the boundary", func(t *testing.T) {
		var paged []*scopedTestResource153
		token := ""
		for {
			page, nextToken, err := svc.ListResourcesWithFilter(ctx, 1, token, keep)
			require.NoError(t, err)
			paged = append(paged, page...)
			if nextToken == "" {
				break
			}
			token = nextToken
		}
		require.Equal(t, want, names(paged))
	})
}

func TestScopeAwareServiceWrapper_Resources(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		svc := newScopeAwareWrapperForTest(t)
		require.Empty(t, collectStream(t, svc.Resources(t.Context(), "", "")))
	})

	t.Run("orders unscoped before scoped and honors the cursor range", func(t *testing.T) {
		svc := newScopeAwareWrapperForTest(t)
		ctx := t.Context()

		want := []scopes.QualifiedName{
			{Name: "a"},
			{Name: "b"},
			{Scope: "/security", Name: "x"},
			{Scope: "/security/eu", Name: "y"},
		}
		for _, qn := range want {
			_, err := svc.CreateResource(ctx, newScopedTestResource153(qn))
			require.NoError(t, err)
		}

		// The full range yields everything, unscoped first.
		require.Equal(t, want, names(collectStream(t, svc.Resources(ctx, "", ""))))

		// The scoped-start cursor is the boundary between the unscoped and scoped
		// halves of the unified stream.
		unscopedOnly := collectStream(t, svc.Resources(ctx, "", scopes.ResourceCursorScopedStart()))
		require.Equal(t, []scopes.QualifiedName{{Name: "a"}, {Name: "b"}}, names(unscopedOnly))

		scopedOnly := collectStream(t, svc.Resources(ctx, scopes.ResourceCursorScopedStart(), ""))
		require.Equal(t, []scopes.QualifiedName{
			{Scope: "/security", Name: "x"},
			{Scope: "/security/eu", Name: "y"},
		}, names(scopedOnly))
	})
}

func TestScopeAwareServiceWrapper_MakeBackendItem(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)

	for _, tc := range []struct {
		name string
		qn   scopes.QualifiedName
	}{
		{"unscoped", scopes.QualifiedName{Name: "foo"}},
		{"scoped", scopes.QualifiedName{Scope: "/security", Name: "foo"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item, err := svc.MakeBackendItem(newScopedTestResource153(tc.qn))
			require.NoError(t, err)

			// The item is keyed exactly where BackendKey reports.
			wantKey, err := svc.BackendKey(tc.qn)
			require.NoError(t, err)
			require.Equal(t, wantKey.String(), item.Key.String())

			// The stored value is the marshaled body and round-trips.
			require.NotEmpty(t, item.Value)
			decoded, err := unmarshalScopedResource153(item.Value)
			require.NoError(t, err)
			requireResourceBody(t, tc.qn, decoded)
		})
	}
}

func TestScopeAwareServiceWrapper_BackendKey(t *testing.T) {
	t.Parallel()
	svc := newScopeAwareWrapperForTest(t)

	t.Run("unscoped", func(t *testing.T) {
		key, err := svc.BackendKey(scopes.QualifiedName{Name: "foo"})
		require.NoError(t, err)
		require.Equal(t, backend.NewKey(testUnscopedPrefix, "foo").String(), key.String())
	})

	t.Run("scoped is namespaced by encoded scope", func(t *testing.T) {
		encodedScope, err := scopes.EncodeForKey("/security")
		require.NoError(t, err)
		key, err := svc.BackendKey(scopes.QualifiedName{Scope: "/security", Name: "foo"})
		require.NoError(t, err)
		require.Equal(t,
			backend.NewKey(testScopedTopPrefix, testUnscopedPrefix, encodedScope, "foo").String(),
			key.String(),
		)
	})
}
