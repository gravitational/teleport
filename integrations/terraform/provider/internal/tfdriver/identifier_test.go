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

package tfdriver_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

type attributeReaderFunc func(path.Path, any) diag.Diagnostics

func (a attributeReaderFunc) GetAttribute(_ context.Context, p path.Path, v any) diag.Diagnostics {
	return a(p, v)
}

func TestNameIdentifierFromPath(t *testing.T) {
	cases := []struct {
		name               string
		attributer         attributeReaderFunc
		expectedIdentifier tfdriver.NameIdentifier
		expectError        bool
	}{
		{
			name: "failed to get attribute",
			attributer: func(path path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				diags.Append(diag.NewErrorDiagnostic("fail", "error"))
				return diags
			},
			expectError: true,
		},
		{
			name: "success",
			attributer: func(path path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				s, ok := v.(*types.String)
				if !ok {
					diags.Append(diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v)))
					return diags
				}

				s.Value = "testing"

				return diag.Diagnostics{}
			},
			expectedIdentifier: tfdriver.NameIdentifier{Name: "testing"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			identifier, d := tfdriver.NameIdentifierFromPath(path.Root("testing"))(t.Context(), test.attributer)
			assert.Equal(t, test.expectError, d.HasError())
			assert.Equal(t, test.expectedIdentifier, identifier)
		})
	}
}

func TestNewNameIdentifier(t *testing.T) {
	identifier, err := tfdriver.NewNameIdentifier("testing")
	require.NoError(t, err)
	require.Equal(t, tfdriver.NameIdentifier{Name: "testing"}, identifier)
}

func TestScopeQualifiedNameIdentifierFromPath(t *testing.T) {
	cases := []struct {
		name               string
		attributer         attributeReaderFunc
		expectedIdentifier tfdriver.ScopeQualifiedNameIdentifier
		expectError        bool
	}{
		{
			name: "failed to get name attribute",
			attributer: func(p path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				s, ok := v.(*types.String)
				if !ok {
					diags.Append(diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v)))
					return diags
				}

				switch {
				case p.Equal(path.Root("name")):
					var diags diag.Diagnostics
					diags.Append(diag.NewErrorDiagnostic("fail", "error"))
					return diags
				case p.Equal(path.Root("scope")):
					s.Value = "/foo/bar"
				}

				return diag.Diagnostics{}
			},
			expectError: true,
		},
		{
			name: "failed to get scope attribute",
			attributer: func(p path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				s, ok := v.(*types.String)
				if !ok {
					diags.Append(diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v)))
					return diags
				}

				switch {
				case p.Equal(path.Root("name")):
					s.Value = "testing"
				case p.Equal(path.Root("scope")):
					var diags diag.Diagnostics
					diags.Append(diag.NewErrorDiagnostic("fail", "error"))
					return diags
				}

				return diag.Diagnostics{}
			},
			expectError: true,
		},
		{
			name: "success",
			attributer: func(p path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				s, ok := v.(*types.String)
				if !ok {
					diags.Append(diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v)))
					return diags
				}

				switch {
				case p.Equal(path.Root("name")):
					s.Value = "testing"
				case p.Equal(path.Root("scope")):
					s.Value = "/foo/bar"
				}

				return diag.Diagnostics{}
			},
			expectedIdentifier: tfdriver.ScopeQualifiedNameIdentifier{Name: "testing", Scope: "/foo/bar"},
		},
		{
			name: "invalid SQN",
			attributer: func(p path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				s, ok := v.(*types.String)
				if !ok {
					diags.Append(diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v)))
					return diags
				}

				switch {
				case p.Equal(path.Root("name")):
					s.Value = "testing"
				case p.Equal(path.Root("scope")):
					s.Value = "/foo/bar&[]/"
				}

				return diag.Diagnostics{}
			},
			expectError: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			identifier, d := tfdriver.ScopeQualifiedNameIdentifierFromPath(path.Root("name"), path.Root("scope"))(t.Context(), test.attributer)
			assert.Equal(t, test.expectError, d.HasError())
			assert.Equal(t, test.expectedIdentifier, identifier)
		})
	}
}

func TestNewScopeQualifiedNameIdentifier(t *testing.T) {
	cases := []struct {
		name               string
		input              string
		errorAssertion     require.ErrorAssertionFunc
		expectedIdentifier tfdriver.ScopeQualifiedNameIdentifier
	}{
		{
			name:               "success",
			input:              "/animals/llama::testing",
			errorAssertion:     require.NoError,
			expectedIdentifier: tfdriver.ScopeQualifiedNameIdentifier{Name: "testing", Scope: "/animals/llama"},
		},
		{
			name:           "invalid SQN format",
			input:          "testing",
			errorAssertion: require.Error,
		},
		{
			name:           "invalid SQN",
			input:          "/animals/llama::testing&[]/",
			errorAssertion: require.Error,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			identifier, err := tfdriver.NewScopedQualifiedNameIdentifier(test.input)
			test.errorAssertion(t, err)
			require.Equal(t, test.expectedIdentifier, identifier)
		})
	}

}

func TestCompositeIdentifierFromPath(t *testing.T) {
	cases := []struct {
		name               string
		attributer         attributeReaderFunc
		expectedIdentifier tfdriver.CompositeIdentifier
		expectError        bool
	}{
		{
			name: "failed to get name attribute",
			attributer: func(p path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				s, ok := v.(*types.String)
				if !ok {
					diags.Append(diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v)))
					return diags
				}

				switch {
				case p.Equal(path.Root("name")):
					var diags diag.Diagnostics
					diags.Append(diag.NewErrorDiagnostic("fail", "error"))
					return diags
				case p.Equal(path.Root("prefix")):
					s.Value = "foo"
				}

				return diag.Diagnostics{}
			},
			expectError: true,
		},
		{
			name: "failed to get prefix attribute",
			attributer: func(p path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				s, ok := v.(*types.String)
				if !ok {
					diags.Append(diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v)))
					return diags
				}

				switch {
				case p.Equal(path.Root("name")):
					s.Value = "testing"
				case p.Equal(path.Root("prefix")):
					var diags diag.Diagnostics
					diags.Append(diag.NewErrorDiagnostic("fail", "error"))
					return diags
				}

				return diag.Diagnostics{}
			},
			expectError: true,
		},
		{
			name: "success",
			attributer: func(p path.Path, v any) diag.Diagnostics {
				var diags diag.Diagnostics
				s, ok := v.(*types.String)
				if !ok {
					diags.Append(diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v)))
					return diags
				}

				switch {
				case p.Equal(path.Root("name")):
					s.Value = "testing"
				case p.Equal(path.Root("prefix")):
					s.Value = "foo"
				}

				return diag.Diagnostics{}
			},
			expectedIdentifier: tfdriver.CompositeIdentifier{Name: "testing", Prefix: "foo"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			identifier, d := tfdriver.CompositeIdentifierFromPath(path.Root("prefix"), path.Root("name"))(t.Context(), test.attributer)
			assert.Equal(t, test.expectError, d.HasError())
			assert.Equal(t, test.expectedIdentifier, identifier)
		})
	}
}

func TestNewCompositeNameIdentifier(t *testing.T) {
	cases := []struct {
		name               string
		input              string
		errorAssertion     require.ErrorAssertionFunc
		expectedIdentifier tfdriver.CompositeIdentifier
	}{
		{
			name:               "success",
			input:              "llama/testing",
			errorAssertion:     require.NoError,
			expectedIdentifier: tfdriver.CompositeIdentifier{Name: "testing", Prefix: "llama"},
		},
		{
			name:           "invalid format",
			input:          "/animals/llama/testing",
			errorAssertion: require.Error,
		},
		{
			name:           "invalid prefix",
			input:          "/llama",
			errorAssertion: require.Error,
		},
		{
			name:           "invalid name",
			input:          "animals/",
			errorAssertion: require.Error,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			identifier, err := tfdriver.NewCompositeIdentifier(test.input)
			test.errorAssertion(t, err)
			require.Equal(t, test.expectedIdentifier, identifier)
		})
	}
}

func TestSingletonIdentifierPolicy(t *testing.T) {
	policy := tfdriver.SingletonIdentifierPolicy[struct{}]("cluster_config")

	fromState, diags := policy.FromState(t.Context(), attributeReaderFunc(func(path.Path, any) diag.Diagnostics {
		t.Fatal("singleton state extraction should not read Terraform attributes")
		return nil
	}))
	require.False(t, diags.HasError(), diags)
	require.Equal(t, tfdriver.SingletonIdentifier{Name: "cluster_config"}, fromState)

	fromResource := policy.FromResource(&struct{}{})
	require.Equal(t, tfdriver.SingletonIdentifier{Name: "cluster_config"}, fromResource)

	fromImport, err := policy.FromImportID("cluster_config")
	require.NoError(t, err)
	require.Equal(t, tfdriver.SingletonIdentifier{Name: "cluster_config"}, fromImport)

	_, err = policy.FromImportID("other")
	require.Error(t, err)
}

func TestNameIdentifierPolicy(t *testing.T) {
	type resource struct{ name string }
	policy := tfdriver.NameIdentifierPolicy(path.Root("name"), func(r *resource) string {
		return r.name
	})

	fromResource := policy.FromResource(&resource{name: "from-resource"})
	require.Equal(t, tfdriver.NameIdentifier{Name: "from-resource"}, fromResource)

	fromImport, err := policy.FromImportID("from-import")
	require.NoError(t, err)
	require.Equal(t, tfdriver.NameIdentifier{Name: "from-import"}, fromImport)
}

func TestCompositeIdentifierPolicy(t *testing.T) {
	type resource struct {
		prefix string
		name   string
	}
	policy := tfdriver.CompositeIdentifierPolicy(
		path.Root("prefix"),
		path.Root("name"),
		func(r *resource) (prefix, name string) {
			return r.prefix, r.name
		},
	)

	fromState, diags := policy.FromState(t.Context(), attributeReaderFunc(func(p path.Path, v any) diag.Diagnostics {
		s, ok := v.(*types.String)
		if !ok {
			return diag.Diagnostics{diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v))}
		}
		switch {
		case p.Equal(path.Root("prefix")):
			s.Value = "state-prefix"
		case p.Equal(path.Root("name")):
			s.Value = "state-name"
		default:
			return diag.Diagnostics{diag.NewErrorDiagnostic("fail", fmt.Sprintf("unexpected path %s", p.String()))}
		}
		return nil
	}))
	require.False(t, diags.HasError(), diags)
	require.Equal(t, tfdriver.CompositeIdentifier{Prefix: "state-prefix", Name: "state-name"}, fromState)

	fromResource := policy.FromResource(&resource{prefix: "resource-prefix", name: "resource-name"})
	require.Equal(t, tfdriver.CompositeIdentifier{Prefix: "resource-prefix", Name: "resource-name"}, fromResource)

	fromImport, err := policy.FromImportID("import-prefix/import-name")
	require.NoError(t, err)
	require.Equal(t, tfdriver.CompositeIdentifier{Prefix: "import-prefix", Name: "import-name"}, fromImport)
}

func TestScopeQualifiedNameIdentifierPolicy(t *testing.T) {
	type resource struct {
		name  string
		scope string
	}
	policy := tfdriver.ScopeQualifiedNameIdentifierPolicy(
		path.Root("name"),
		path.Root("scope"),
		func(r *resource) (name, scope string) {
			return r.name, r.scope
		},
	)

	fromState, diags := policy.FromState(t.Context(), attributeReaderFunc(func(p path.Path, v any) diag.Diagnostics {
		s, ok := v.(*types.String)
		if !ok {
			return diag.Diagnostics{diag.NewErrorDiagnostic("fail", fmt.Sprintf("expected type string, but got %T", v))}
		}
		switch {
		case p.Equal(path.Root("name")):
			s.Value = "state-name"
		case p.Equal(path.Root("scope")):
			s.Value = "/state/scope"
		default:
			return diag.Diagnostics{diag.NewErrorDiagnostic("fail", fmt.Sprintf("unexpected path %s", p.String()))}
		}
		return nil
	}))
	require.False(t, diags.HasError(), diags)
	require.Equal(t, tfdriver.ScopeQualifiedNameIdentifier{Name: "state-name", Scope: "/state/scope"}, fromState)

	fromResource := policy.FromResource(&resource{name: "resource-name", scope: "/resource/scope"})
	require.Equal(t, tfdriver.ScopeQualifiedNameIdentifier{Name: "resource-name", Scope: "/resource/scope"}, fromResource)

	fromImport, err := policy.FromImportID("/import/scope::import-name")
	require.NoError(t, err)
	require.Equal(t, tfdriver.ScopeQualifiedNameIdentifier{Name: "import-name", Scope: "/import/scope"}, fromImport)
}

func TestNameIdentifierPolicyFromState(t *testing.T) {
	type resource struct{ name string }
	policy := tfdriver.NameIdentifierPolicy(path.Root("name"), func(r *resource) string {
		return r.name
	})

	fromState, diags := policy.FromState(t.Context(), attributeReaderFunc(func(p path.Path, v any) diag.Diagnostics {
		require.True(t, p.Equal(path.Root("name")))
		s, ok := v.(*types.String)
		require.True(t, ok)
		s.Value = "state-name"
		return nil
	}))
	require.False(t, diags.HasError(), diags)
	require.Equal(t, tfdriver.NameIdentifier{Name: "state-name"}, fromState)
}
