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

package tfgen

import "github.com/gravitational/teleport/lib/tfgen/internal"

// GenerateOpt is an optional argument that can be passed to customize the
// behavior of Generate.
type GenerateOpt func(*generateOpts)

// WithFieldTransform applies a transformation to the given field. It is used
// when our Terraform representation of the field diverges from the protobuf
// structure (see: transform.BotTraits for an example).
//
// name is the dot-syntax path to the field (e.g. `spec.traits`).
func WithFieldTransform(name string, transform Transform) GenerateOpt {
	return func(o *generateOpts) { o.fieldTransforms[name] = transform }
}

// Transform that can be used to change the Terraform representation of a field.
type Transform func(*internal.Value) (*internal.Value, error)

// WithFieldComment adds a comment above the given field.
//
// If the field is unpopulated, its zero value will be emitted so that the
// comment is preserved.
//
// name is the dot-syntax path to the field (e.g. `spec.traits`).
//
// Note: currently only single-line comments on fields within the spec and
// metadata objects are supported. You cannot add comments to top-level fields
// such as version, sub_kind, or the metadata and spec fields themselves. You
// also cannot add comments to objects within a list or map.
func WithFieldComment(name, comment string) GenerateOpt {
	return func(o *generateOpts) { o.fieldComments[name] = comment }
}

// WithResourceType overrides the Terraform resource type. By default, the type
// will be "teleport_<kind>".
func WithResourceType(typeName string) GenerateOpt {
	return func(o *generateOpts) { o.resourceType = typeName }
}

// WithResourceName overrides the Terraform resource name. By default, the name
// will be taken from the resource's metadata.
func WithResourceName(name string) GenerateOpt {
	return func(o *generateOpts) { o.resourceName = name }
}

// WithResourceBlockComment adds a comment before the resource block.
func WithResourceBlockComment(comment string) GenerateOpt {
	return func(o *generateOpts) { o.resourceBlockComment = comment }
}

// WithDependsOn adds a terraform depends_on meta-argument to the generated
// resource block. The value should be a reference to another Terraform resource
// in the form "resource_type.resource_name".
// E.g. "teleport_access_list.acl-UID1234" means the generated resource will
// wait for "teleport_access_list.acl-UID1234" to be created first.
//
// May be called multiple times to add multiple references.
func WithDependsOn(reference string) GenerateOpt {
	return func(o *generateOpts) { o.dependsOn = append(o.dependsOn, reference) }
}

// WithOmitField omits the given field from the generated output.
//
// "name" is the dot-syntax path to the field to omit e.g.:
//   - `spec.ineligible_status`
//   - `header.metadata.description`
//
// Can omit fields from every item in a list e.g.:
//   - `spec.owners.description`: omits "description" field per owner
//
// Can omit fields in nested lists e.g.:
//   - `spec.mappings.match.group_ids`: omits "group_ids" field per match per mappings
func WithOmitField(name string) GenerateOpt {
	return func(o *generateOpts) { o.fieldsToOmit[name] = true }
}

type generateOpts struct {
	resourceType         string
	resourceName         string
	resourceBlockComment string
	dependsOn            []string

	fieldTransforms map[string]Transform
	fieldComments   map[string]string
	fieldsToOmit    map[string]bool
}

func newGenerateOpts() *generateOpts {
	return &generateOpts{
		fieldTransforms: make(map[string]Transform),
		fieldComments:   make(map[string]string),
		fieldsToOmit:    make(map[string]bool),
	}
}
