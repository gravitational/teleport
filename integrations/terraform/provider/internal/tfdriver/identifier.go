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

package tfdriver

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gravitational/teleport/lib/scopes"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdiag"
)

// Identifier is a stable Terraform and Teleport identifier.
type Identifier interface {
	fmt.Stringer
}

// IdentifierParser parses a Terraform import ID into a typed identifier.
type IdentifierParser[I Identifier] func(string) (I, error)

// TerraformIdentifierExtractor extracts an identifier from Terraform values.
type TerraformIdentifierExtractor[I Identifier] func(context.Context, TerraformAttributeReader) (I, diag.Diagnostics)

// ResourceIdentifierExtractor extracts an identifier from a Teleport resource.
type ResourceIdentifierExtractor[T any, I Identifier] func(*T) I

// IdentifierPolicy describes how to get a resource identifier.
type IdentifierPolicy[T any, I Identifier] struct {
	FromState    TerraformIdentifierExtractor[I]
	FromResource ResourceIdentifierExtractor[T, I]
	FromImportID IdentifierParser[I]
}

// NameIdentifier identifies a resource by name.
type NameIdentifier struct {
	Name string
}

// String returns the Terraform import ID.
func (n NameIdentifier) String() string {
	return n.Name
}

// ScopeQualifiedNameIdentifier identifies a resource by scope and name.
type ScopeQualifiedNameIdentifier struct {
	Name  string
	Scope string
}

// String returns the Terraform import ID.
func (n ScopeQualifiedNameIdentifier) String() string {
	return scopes.QualifiedName{Name: n.Name, Scope: n.Scope}.String()
}

// CompositeIdentifier identifies a resource by prefix and name.
type CompositeIdentifier struct {
	Prefix string
	Name   string
}

// String returns the Terraform import ID.
func (n CompositeIdentifier) String() string {
	return n.Prefix + "/" + n.Name
}

// ScopeQualifiedCompositeIdentifier identifies a resource by a possibly
// scope-qualified prefix and a possibly scope-qualified name.
type ScopeQualifiedCompositeIdentifier struct {
	Prefix ScopeQualifiedNameIdentifier
	Name   ScopeQualifiedNameIdentifier
}

// String returns the Terraform import ID.
func (n ScopeQualifiedCompositeIdentifier) String() string {
	return n.Prefix.String() + "/" + n.Name.String()
}

// SingletonIdentifier identifies a resource with a fixed name.
type SingletonIdentifier struct {
	Name string
}

// String returns the Terraform import ID.
func (n SingletonIdentifier) String() string {
	return n.Name
}

// TerraformAttributeReader reads attributes from Terraform values.
type TerraformAttributeReader interface {
	// GetAttribute reads one Terraform attribute.
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

// NewNameIdentifier returns a name identifier.
func NewNameIdentifier(s string) (NameIdentifier, error) {
	return NameIdentifier{Name: s}, nil
}

// NameIdentifierFromPath returns an extractor for a name identifier.
func NameIdentifierFromPath(p path.Path) TerraformIdentifierExtractor[NameIdentifier] {
	return func(ctx context.Context, reader TerraformAttributeReader) (NameIdentifier, diag.Diagnostics) {
		var identifier types.String
		diags := reader.GetAttribute(ctx, p, &identifier)
		if diags.HasError() {
			return NameIdentifier{}, diags
		}

		return NameIdentifier{Name: identifier.Value}, diags
	}
}

// NameIdentifierPolicy returns an identifier policy for name identifiers.
func NameIdentifierPolicy[T any](statePath path.Path, resourceName func(*T) string) IdentifierPolicy[T, NameIdentifier] {
	return IdentifierPolicy[T, NameIdentifier]{
		FromState: NameIdentifierFromPath(statePath),
		FromResource: func(resource *T) NameIdentifier {
			return NameIdentifier{Name: resourceName(resource)}
		},
		FromImportID: NewNameIdentifier,
	}
}

// NewScopedQualifiedNameIdentifier parses a scope qualified name identifier.
func NewScopedQualifiedNameIdentifier(s string) (ScopeQualifiedNameIdentifier, error) {
	sqn, err := scopes.ParseQualifiedName(s)
	if err != nil {
		return ScopeQualifiedNameIdentifier{}, trace.Wrap(err)
	}

	if err := sqn.StrongValidate(); err != nil {
		return ScopeQualifiedNameIdentifier{}, trace.Wrap(err)
	}

	return ScopeQualifiedNameIdentifier{Name: sqn.Name, Scope: sqn.Scope}, nil
}

// NewPossiblyUnscopedScopeQualifiedNameIdentifier parses an identifier that may
// be either a scope-qualified name or an unscoped bare name.
func NewPossiblyUnscopedScopeQualifiedNameIdentifier(s string) (ScopeQualifiedNameIdentifier, error) {
	if strings.Contains(s, scopes.QualifiedNameSeparator) {
		return NewScopedQualifiedNameIdentifier(s)
	}

	return ScopeQualifiedNameIdentifier{Name: s}, nil
}

// ScopeQualifiedNameIdentifierFromPath returns a scope qualified name extractor.
func ScopeQualifiedNameIdentifierFromPath(namePath, scopePath path.Path) TerraformIdentifierExtractor[ScopeQualifiedNameIdentifier] {
	return func(ctx context.Context, reader TerraformAttributeReader) (ScopeQualifiedNameIdentifier, diag.Diagnostics) {
		var identifier types.String
		diags := reader.GetAttribute(ctx, namePath, &identifier)
		if diags.HasError() {
			return ScopeQualifiedNameIdentifier{}, diags
		}

		var scope types.String
		diags.Append(reader.GetAttribute(ctx, scopePath, &scope)...)
		if diags.HasError() {
			return ScopeQualifiedNameIdentifier{}, diags
		}

		sqn := scopes.QualifiedName{Name: identifier.Value, Scope: scope.Value}
		if err := sqn.WeakValidate(); err != nil {
			diags.Append(tfdiag.DiagFromErr("malformed scope qualified name", err))
			return ScopeQualifiedNameIdentifier{}, diags
		}

		return ScopeQualifiedNameIdentifier{Name: sqn.Name, Scope: sqn.Scope}, diags
	}
}

// PossiblyUnscopedScopeQualifiedNameIdentifierFromPath returns an extractor for
// identifiers that may be either scope-qualified or unscoped.
func PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(namePath, scopePath path.Path) TerraformIdentifierExtractor[ScopeQualifiedNameIdentifier] {
	return func(ctx context.Context, reader TerraformAttributeReader) (ScopeQualifiedNameIdentifier, diag.Diagnostics) {
		var identifier types.String
		diags := reader.GetAttribute(ctx, namePath, &identifier)
		if diags.HasError() {
			return ScopeQualifiedNameIdentifier{}, diags
		}

		var scope types.String
		diags.Append(reader.GetAttribute(ctx, scopePath, &scope)...)
		if diags.HasError() {
			return ScopeQualifiedNameIdentifier{}, diags
		}

		if scope.Null || scope.Unknown || scope.Value == "" {
			return ScopeQualifiedNameIdentifier{Name: identifier.Value}, diags
		}

		sqn := scopes.QualifiedName{Name: identifier.Value, Scope: scope.Value}
		if err := sqn.WeakValidate(); err != nil {
			diags.Append(tfdiag.DiagFromErr("malformed scope qualified name", err))
			return ScopeQualifiedNameIdentifier{}, diags
		}

		return ScopeQualifiedNameIdentifier{Name: sqn.Name, Scope: sqn.Scope}, diags
	}
}

// ScopeQualifiedNameIdentifierPolicy returns a policy for scope qualified names.
func ScopeQualifiedNameIdentifierPolicy[T any](namePath, scopePath path.Path, resourceNameAndScope func(*T) (name, scope string)) IdentifierPolicy[T, ScopeQualifiedNameIdentifier] {
	return IdentifierPolicy[T, ScopeQualifiedNameIdentifier]{
		FromState: ScopeQualifiedNameIdentifierFromPath(namePath, scopePath),
		FromResource: func(resource *T) ScopeQualifiedNameIdentifier {
			name, scope := resourceNameAndScope(resource)
			return ScopeQualifiedNameIdentifier{Name: name, Scope: scope}
		},
		FromImportID: NewScopedQualifiedNameIdentifier,
	}
}

// PossiblyUnscopedScopeQualifiedNameIdentifierPolicy returns a policy for
// identifiers that may be either scope-qualified or unscoped.
func PossiblyUnscopedScopeQualifiedNameIdentifierPolicy[T any](namePath, scopePath path.Path, resourceNameAndScope func(*T) (name, scope string)) IdentifierPolicy[T, ScopeQualifiedNameIdentifier] {
	return IdentifierPolicy[T, ScopeQualifiedNameIdentifier]{
		FromState: PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(namePath, scopePath),
		FromResource: func(resource *T) ScopeQualifiedNameIdentifier {
			name, scope := resourceNameAndScope(resource)
			return ScopeQualifiedNameIdentifier{Name: name, Scope: scope}
		},
		FromImportID: NewPossiblyUnscopedScopeQualifiedNameIdentifier,
	}
}

// NewCompositeIdentifier parses a composite identifier.
func NewCompositeIdentifier(s string) (CompositeIdentifier, error) {
	split := strings.Split(s, "/")
	if len(split) != 2 {
		return CompositeIdentifier{}, trace.BadParameter("expected id %q to have a single %q separator", s, "/")
	}
	prefix := split[0]
	name := split[1]

	if prefix == "" {
		return CompositeIdentifier{}, trace.BadParameter("expected id %q prefix to be non-empty", s)
	}

	if name == "" {
		return CompositeIdentifier{}, trace.BadParameter("expected id %q name to be non-empty", s)
	}

	return CompositeIdentifier{Prefix: prefix, Name: name}, nil
}

// CompositeIdentifierFromPath returns an extractor for a composite identifier.
func CompositeIdentifierFromPath(prefixPath, namePath path.Path) TerraformIdentifierExtractor[CompositeIdentifier] {
	return func(ctx context.Context, reader TerraformAttributeReader) (CompositeIdentifier, diag.Diagnostics) {
		var prefix types.String
		diags := reader.GetAttribute(ctx, prefixPath, &prefix)
		if diags.HasError() {
			return CompositeIdentifier{}, diags
		}

		var id types.String
		diags.Append(reader.GetAttribute(ctx, namePath, &id)...)
		if diags.HasError() {
			return CompositeIdentifier{}, diags
		}

		return CompositeIdentifier{Prefix: prefix.Value, Name: id.Value}, diags
	}
}

// CompositeIdentifierPolicy returns an identifier policy for composite identifiers.
func CompositeIdentifierPolicy[T any](prefixPath, namePath path.Path, resourcePrefixAndName func(*T) (prefix, name string)) IdentifierPolicy[T, CompositeIdentifier] {
	return IdentifierPolicy[T, CompositeIdentifier]{
		FromState: CompositeIdentifierFromPath(prefixPath, namePath),
		FromResource: func(resource *T) CompositeIdentifier {
			prefix, name := resourcePrefixAndName(resource)
			return CompositeIdentifier{Prefix: prefix, Name: name}
		},
		FromImportID: NewCompositeIdentifier,
	}
}

// NewScopeQualifiedCompositeIdentifier parses a scope-qualified composite
// identifier in "prefix/name" form, where either side may be a scope-qualified
// name. For example: "access-list/alice", "/scope::access-list/alice", or
// "/scope::access-list//scope::child-list".
func NewScopeQualifiedCompositeIdentifier(s string) (ScopeQualifiedCompositeIdentifier, error) {
	prefix, name, err := splitScopeQualifiedCompositeIdentifier(s)
	if err != nil {
		return ScopeQualifiedCompositeIdentifier{}, trace.Wrap(err)
	}

	return newScopeQualifiedCompositeIdentifier(prefix, name)
}

func splitScopeQualifiedCompositeIdentifier(s string) (prefix, name string, err error) {
	if strings.HasPrefix(s, "/") {
		scope, rest, ok := strings.Cut(s, scopes.QualifiedNameSeparator)
		if !ok {
			return "", "", trace.BadParameter("expected scoped composite id %q prefix to contain %q", s, scopes.QualifiedNameSeparator)
		}

		prefixName, name, ok := strings.Cut(rest, "/")
		if !ok {
			return "", "", trace.BadParameter("expected id %q to have a %q separator after prefix", s, "/")
		}
		return scope + scopes.QualifiedNameSeparator + prefixName, name, nil
	}

	prefix, name, ok := strings.Cut(s, "/")
	if !ok {
		return "", "", trace.BadParameter("expected id %q to have a %q separator", s, "/")
	}

	if strings.Contains(name, "/") && !strings.HasPrefix(name, "/") {
		return "", "", trace.BadParameter("expected unscoped member name in id %q not to contain %q", s, "/")
	}

	return prefix, name, nil
}

func newScopeQualifiedCompositeIdentifier(prefix, name string) (ScopeQualifiedCompositeIdentifier, error) {
	if prefix == "" {
		return ScopeQualifiedCompositeIdentifier{}, trace.BadParameter("prefix must be non-empty")
	}
	if name == "" {
		return ScopeQualifiedCompositeIdentifier{}, trace.BadParameter("name must be non-empty")
	}

	prefixSQN, err := NewPossiblyUnscopedScopeQualifiedNameIdentifier(prefix)
	if err != nil {
		return ScopeQualifiedCompositeIdentifier{}, trace.Wrap(err)
	}

	nameSQN, err := NewPossiblyUnscopedScopeQualifiedNameIdentifier(name)
	if err != nil {
		return ScopeQualifiedCompositeIdentifier{}, trace.Wrap(err)
	}

	return ScopeQualifiedCompositeIdentifier{Prefix: prefixSQN, Name: nameSQN}, nil
}

// ScopeQualifiedCompositeIdentifierFromPath returns an extractor for a
// scope-qualified composite identifier.
func ScopeQualifiedCompositeIdentifierFromPath(prefixPath, namePath path.Path) TerraformIdentifierExtractor[ScopeQualifiedCompositeIdentifier] {
	return func(ctx context.Context, reader TerraformAttributeReader) (ScopeQualifiedCompositeIdentifier, diag.Diagnostics) {
		var prefix types.String
		diags := reader.GetAttribute(ctx, prefixPath, &prefix)
		if diags.HasError() {
			return ScopeQualifiedCompositeIdentifier{}, diags
		}

		var name types.String
		diags.Append(reader.GetAttribute(ctx, namePath, &name)...)
		if diags.HasError() {
			return ScopeQualifiedCompositeIdentifier{}, diags
		}

		prefixSQN, err := scopeQualifiedNameIdentifierFromPossiblyQualifiedString(prefix.Value)
		if err != nil {
			diags.Append(tfdiag.DiagFromErr("malformed scope qualified prefix", err))
			return ScopeQualifiedCompositeIdentifier{}, diags
		}

		nameSQN, err := scopeQualifiedNameIdentifierFromPossiblyQualifiedString(name.Value)
		if err != nil {
			diags.Append(tfdiag.DiagFromErr("malformed scope qualified name", err))
			return ScopeQualifiedCompositeIdentifier{}, diags
		}

		return ScopeQualifiedCompositeIdentifier{Prefix: prefixSQN, Name: nameSQN}, diags
	}
}

func scopeQualifiedNameIdentifierFromPossiblyQualifiedString(s string) (ScopeQualifiedNameIdentifier, error) {
	if !strings.Contains(s, scopes.QualifiedNameSeparator) {
		return ScopeQualifiedNameIdentifier{Name: s}, nil
	}

	sqn, err := scopes.ParseQualifiedName(s)
	if err != nil {
		return ScopeQualifiedNameIdentifier{}, trace.Wrap(err)
	}

	if err := sqn.WeakValidate(); err != nil {
		return ScopeQualifiedNameIdentifier{}, trace.Wrap(err)
	}

	return ScopeQualifiedNameIdentifier{Name: sqn.Name, Scope: sqn.Scope}, nil
}

// ScopeQualifiedCompositeIdentifierPolicy returns a policy for scope-qualified
// composite identifiers.
func ScopeQualifiedCompositeIdentifierPolicy[T any](prefixPath, namePath path.Path, resourcePrefixAndName func(*T) (prefix, name string)) IdentifierPolicy[T, ScopeQualifiedCompositeIdentifier] {
	return IdentifierPolicy[T, ScopeQualifiedCompositeIdentifier]{
		FromState: ScopeQualifiedCompositeIdentifierFromPath(prefixPath, namePath),
		FromResource: func(resource *T) ScopeQualifiedCompositeIdentifier {
			prefix, name := resourcePrefixAndName(resource)
			prefixID, _ := scopeQualifiedNameIdentifierFromPossiblyQualifiedString(prefix)
			nameID, _ := scopeQualifiedNameIdentifierFromPossiblyQualifiedString(name)
			return ScopeQualifiedCompositeIdentifier{Prefix: prefixID, Name: nameID}
		},
		FromImportID: NewScopeQualifiedCompositeIdentifier,
	}
}

// NewSingletonIdentifier returns a singleton identifier.
func NewSingletonIdentifier(s string) (SingletonIdentifier, error) {
	return SingletonIdentifier{Name: s}, nil
}

// SingletonImportIdentifier returns a parser for a singleton import ID.
func SingletonImportIdentifier(name string) IdentifierParser[SingletonIdentifier] {
	return func(id string) (SingletonIdentifier, error) {
		if id != name {
			return SingletonIdentifier{}, trace.BadParameter("expected singleton id %q, got %q", name, id)
		}
		return NewSingletonIdentifier(name)
	}
}

// SingletonIdentifierFromName returns an extractor for a singleton identifier.
func SingletonIdentifierFromName(name string) TerraformIdentifierExtractor[SingletonIdentifier] {
	return func(context.Context, TerraformAttributeReader) (SingletonIdentifier, diag.Diagnostics) {
		return SingletonIdentifier{Name: name}, nil
	}
}

// SingletonIdentifierPolicy returns an identifier policy for singleton identifiers.
func SingletonIdentifierPolicy[T any](name string) IdentifierPolicy[T, SingletonIdentifier] {
	return IdentifierPolicy[T, SingletonIdentifier]{
		FromState: SingletonIdentifierFromName(name),
		FromResource: func(*T) SingletonIdentifier {
			return SingletonIdentifier{Name: name}
		},
		FromImportID: SingletonImportIdentifier(name),
	}
}
