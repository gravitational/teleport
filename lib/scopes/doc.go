/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

// The scopes package provides helpers for validating and evaluating the scoping of resources and access-control policies. The core principals of scoping are as follows:
//
//   - A scope is a path-like string with segments separated by slashes (e.g. '/foo/bar').
//   - The outermost scope is the root scope ('/').
//   - Given any two valid scopes, they will either be equivalent ('/foo' and '/foo'), orthogonal ('/foo' and '/bar'), or have an ancestore/descendant relationship (e.g. '/foo' and '/foo/bar/bin').
//   - Resources are assigned to exactly one scope.
//   - Policies/permissions are assigned at a scope, but apply to that scope and all descendant scopes (e.g. 'scoped_token:create' in '/foo' implies 'scoped_token:create' in '/foo/bar').
//   - Access-control decision evaluation starts at the root scope, and halts at the first sub-scope where an allow decision is reached (e.g. when evaluating access to a resource in '/foo/bar', an allow decision in '/foo' will ignore policies assigned at '/foo/bar').
//   - A policy/permission is conceptually distinct from the resource that defines it. Assignment of a policy/permission may happen at the same scope as the resource that defines it, or at a child of that scope.
//
// General usage guidance:
//
//   - When reading in a scope value from an external source (user input, identity provider, etc) check it with [StrongValidate].
//   - When reading in a scope value from a trusted source (control-plane, backend, etc) check it with [WeakValidate].
//   - When constructing a scope from component strings (e.g. during string interpolation), call [StrongValidateSegment] on each segment in addition to calling [StrongValidate] on the final scope.
//   - If you have a resource at a given scope and want to filter for applicable policies/permissions, use [ResourceScope].
//   - If you have a policy/permission at a given scope and want to filter for subject resources, use [PolicyScope].
//   - Avoid using [Compare] and the associated [Relationship] type directly in access-control logic.
package scopes
