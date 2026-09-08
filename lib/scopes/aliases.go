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

package scopes

import (
	apiscopes "github.com/gravitational/teleport/api/scopes"
)

// separator is the character used to separate segments in a scope and is the value of the root scope.
const separator = "/"

type (
	// QualifiedName is an alias of [apiscopes.QualifiedName].
	QualifiedName = apiscopes.QualifiedName

	// Relationship is an alias of [apiscopes.Relationship].
	Relationship = apiscopes.Relationship

	// ScopeOfOrigin is an alias of [apiscopes.ScopeOfOrigin].
	ScopeOfOrigin = apiscopes.ScopeOfOrigin

	// ScopeOfEffect is an alias of [apiscopes.ScopeOfEffect].
	ScopeOfEffect = apiscopes.ScopeOfEffect

	// ResourceScope is an alias of [apiscopes.ResourceScope].
	ResourceScope = apiscopes.ResourceScope

	// PolicyResourceScope is an alias of [apiscopes.PolicyResourceScope].
	PolicyResourceScope = apiscopes.PolicyResourceScope

	// ScopeOfEffectGlob is an alias of [apiscopes.ScopeOfEffectGlob].
	ScopeOfEffectGlob = apiscopes.ScopeOfEffectGlob

	// Glob is an alias of [apiscopes.Glob].
	Glob = apiscopes.Glob

	// EnforcementPoint is an alias of [apiscopes.EnforcementPoint].
	EnforcementPoint = apiscopes.EnforcementPoint
)

const (
	// Root is an alias of [apiscopes.Root].
	Root = apiscopes.Root

	// Unscoped is an alias of [apiscopes.Unscoped].
	Unscoped = apiscopes.Unscoped

	// Orthogonal is an alias of [apiscopes.Orthogonal].
	Orthogonal = apiscopes.Orthogonal

	// Equivalent is an alias of [apiscopes.Equivalent].
	Equivalent = apiscopes.Equivalent

	// Ancestor is an alias of [apiscopes.Ancestor].
	Ancestor = apiscopes.Ancestor

	// Descendant is an alias of [apiscopes.Descendant].
	Descendant = apiscopes.Descendant
)

var (
	// StrongValidate is an alias of [apiscopes.StrongValidate].
	StrongValidate = apiscopes.StrongValidate

	// WeakValidate is an alias of [apiscopes.WeakValidate].
	WeakValidate = apiscopes.WeakValidate

	// StrongValidateSegment is an alias of [apiscopes.StrongValidateSegment].
	StrongValidateSegment = apiscopes.StrongValidateSegment

	// WeakValidateSegment is an alias of [apiscopes.WeakValidateSegment].
	WeakValidateSegment = apiscopes.WeakValidateSegment

	// StrongValidateResourceName is an alias of [apiscopes.StrongValidateResourceName].
	StrongValidateResourceName = apiscopes.StrongValidateResourceName

	// StrongValidateGlob is an alias of [apiscopes.StrongValidateGlob].
	StrongValidateGlob = apiscopes.StrongValidateGlob

	// WeakValidateGlob is an alias of [apiscopes.WeakValidateGlob].
	WeakValidateGlob = apiscopes.WeakValidateGlob

	// DescendingSegments is an alias of [apiscopes.DescendingSegments].
	DescendingSegments = apiscopes.DescendingSegments

	// DescendingScopes is an alias of [apiscopes.DescendingScopes].
	DescendingScopes = apiscopes.DescendingScopes

	// AscendingScopes is an alias of [apiscopes.AscendingScopes].
	AscendingScopes = apiscopes.AscendingScopes

	// Split is an alias of [apiscopes.Split].
	Split = apiscopes.Split

	// Depth is an alias of [apiscopes.Depth].
	Depth = apiscopes.Depth

	// Join is an alias of [apiscopes.Join].
	Join = apiscopes.Join

	// NormalizeForEquality is an alias of [apiscopes.NormalizeForEquality].
	NormalizeForEquality = apiscopes.NormalizeForEquality

	// Compare is an alias of [apiscopes.Compare].
	Compare = apiscopes.Compare

	// Sort is an alias of [apiscopes.Sort].
	Sort = apiscopes.Sort

	// EnforcementPointsForResourceScope is an alias of [apiscopes.EnforcementPointsForResourceScope].
	EnforcementPointsForResourceScope = apiscopes.EnforcementPointsForResourceScope

	// ParseQualifiedName is an alias of [apiscopes.ParseQualifiedName].
	ParseQualifiedName = apiscopes.ParseQualifiedName

	// ParseOptionallyQualifiedName is an alias of [apiscopes.ParseOptionallyQualifiedName].
	ParseOptionallyQualifiedName = apiscopes.ParseOptionallyQualifiedName
)
