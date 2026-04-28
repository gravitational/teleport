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
	"go/format"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// rewriteGeneratedFile is the unit under test. These cases focus on the
// individual transforms; an integration test against the live access-graph
// output would be more thorough but also more brittle, so we stick to
// representative fragments here.

func TestRewriteGeneratedFile_ClientPackageAndImports(t *testing.T) {
	t.Parallel()

	src := `package api

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/oapi-codegen/runtime"
	externalRef0 "github.com/gravitational/access-graph/pkg/api/models/graph"
)

func Use(_ context.Context, _ openapi_types.UUID, _ runtime.ParamLocation, _ externalRef0.Foo) {}
`

	got := rewrite(t, src, rewriteClient)

	require.Contains(t, got, "package accessgraph")
	require.Contains(t, got, `"github.com/google/uuid"`)
	require.Contains(t, got, `"github.com/gravitational/teleport/lib/accessgraph/apiclient/models/graph"`)
	require.NotContains(t, got, "openapi_types")
	require.NotContains(t, got, `"github.com/oapi-codegen/runtime/types"`)
	require.NotContains(t, got, `"github.com/oapi-codegen/runtime"`)
	require.Contains(t, got, "uuid.UUID")
}

func TestRewriteGeneratedFile_ModelPreservesPackageAndRetargetsRuntime(t *testing.T) {
	t.Parallel()

	src := `package graph

import (
	"github.com/oapi-codegen/runtime"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type T struct {
	ID openapi_types.UUID
}

var _ = runtime.ParamLocationQuery
`

	got := rewrite(t, src, rewriteModel)

	require.Contains(t, got, "package graph")
	require.Contains(t, got, `runtime "github.com/gravitational/teleport/lib/accessgraph/apiclient/runtime"`)
	require.Contains(t, got, `"github.com/google/uuid"`)
	require.Contains(t, got, "uuid.UUID")
	// Model files keep referencing runtime.* — only the import path moves.
	require.Contains(t, got, "runtime.ParamLocationQuery")
}

func TestRewriteGeneratedFile_PathParamInlined(t *testing.T) {
	t.Parallel()

	src := `package api

import (
	"net/url"

	"github.com/oapi-codegen/runtime"
)

func F(id string) string {
	var err error
	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithOptions("simple", false, "id", id, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string", Format: ""})
	if err != nil {
		return ""
	}
	_ = pathParam0
	return url.PathEscape(pathParam0)
}
`

	got := rewrite(t, src, rewriteClient)

	require.Contains(t, got, "pathParam0 := url.PathEscape(id)")
	require.NotContains(t, got, "StyleParamWithOptions")
}

func TestRewriteGeneratedFile_QueryParamInlined(t *testing.T) {
	t.Parallel()

	// Each case exercises one of the type/format branches in
	// queryValueExpr. We assert on the converted form rather than the
	// full output so the test stays readable; "StyleParamWithOptions
	// must not survive" is asserted globally for every case.
	cases := []struct {
		name      string
		paramType string
		format    string
		paramExpr string
		want      string
	}{
		{
			name:      "integer uses strconv.Itoa",
			paramType: "integer",
			paramExpr: "v int",
			want:      `queryValues.Add("v", strconv.Itoa(v))`,
		},
		{
			name:      "number uses strconv.FormatFloat",
			paramType: "number",
			format:    "float",
			paramExpr: "v float32",
			want:      `queryValues.Add("v", strconv.FormatFloat(float64(v), 'f', -1, 32))`,
		},
		{
			name:      "date-time uses time.RFC3339Nano",
			paramType: "string",
			format:    "date-time",
			paramExpr: "v time.Time",
			want:      `queryValues.Add("v", v.Format(time.RFC3339Nano))`,
		},
		{
			name:      "string falls back to string conversion",
			paramType: "string",
			paramExpr: "v MyString",
			want:      `queryValues.Add("v", string(v))`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			src := `package api

import (
	"net/url"
	"time"

	"github.com/oapi-codegen/runtime"
)

type MyString string

var _ = time.RFC3339Nano

func F(queryValues url.Values, ` + tc.paramExpr + `) {
	if queryFrag, err := runtime.StyleParamWithOptions("form", true, "v", v, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationQuery, Type: "` + tc.paramType + `", Format: "` + tc.format + `"}); err != nil {
		return
	} else if parsed, err := url.ParseQuery(queryFrag); err != nil {
		return
	} else {
		_ = parsed
	}
	_ = queryValues
}
`

			got := rewrite(t, src, rewriteClient)

			require.Contains(t, got, tc.want)
			require.NotContains(t, got, "StyleParamWithOptions")
			require.Contains(t, got, `"strconv"`)
		})
	}
}

func TestRewriteGeneratedFile_RejectsUnconvertedStyleParam(t *testing.T) {
	t.Parallel()

	// A StyleParamWithOptions call that doesn't match either the path-param
	// or query-param shape must surface an error rather than silently
	// emitting code that won't compile (runtime import has been removed).
	src := `package api

import "github.com/oapi-codegen/runtime"

func F(v string) {
	_, _ = runtime.StyleParamWithOptions("matrix", false, "x", v, runtime.StyleParamOptions{ParamLocation: runtime.ParamLocationPath, Type: "string"})
}
`

	_, err := rewriteGeneratedFile([]byte(src), rewriteClient)
	require.ErrorContains(t, err, "unconverted StyleParamWithOptions")
}

// rewrite runs rewriteGeneratedFile and gofmts the result so substring
// assertions don't have to care about whitespace.
func rewrite(t *testing.T, src string, mode rewriteMode) string {
	t.Helper()

	out, err := rewriteGeneratedFile([]byte(src), mode)
	require.NoError(t, err)

	formatted, err := format.Source(out)
	require.NoError(t, err)

	// Collapse runs of whitespace so assertions like
	// `pathParam0 := url.PathEscape(id)` work regardless of how gofmt
	// chose to lay the line out.
	return strings.Join(strings.Fields(string(formatted)), " ")
}
