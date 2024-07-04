/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package teleportequal

import (
	"go/types"
	"strings"

	"github.com/awalterschulze/goderive/derive"
	"github.com/awalterschulze/goderive/plugin/equal"
)

// NewPlugin will create a Teleport equals plugin. This is the default derive equals plugin
// with protobuf fields, ID/Revision, and Status fields filtered out.
func NewPlugin() derive.Plugin {
	return derive.NewPlugin("teleport-equal", "deriveTeleportEqual", New)
}

func New(typesMap derive.TypesMap, p derive.Printer, deps map[string]derive.Dependency) derive.Generator {
	gen := &gen{
		TypesMap: typesMap,
	}
	gen.equalGen = equal.New(gen, p, deps)
	return gen
}

type gen struct {
	derive.TypesMap
	equalGen derive.Generator
}

func (g *gen) Add(name string, typs []types.Type) (string, error) {
	return g.equalGen.Add(name, typs)
}

func (g *gen) Generate(typs []types.Type) error {
	return g.equalGen.Generate(typs)
}

func (g *gen) ToGenerate() [][]types.Type {
	return filterTypes(g.TypesMap.ToGenerate())
}

// filterTypes will take the given types from ToGenerate and filter out fields from
// messages that we want to ignore.
func filterTypes(typsOfTyps [][]types.Type) [][]types.Type {
	for i, typs := range typsOfTyps {
		for j, typ := range typs {
			typsOfTyps[i][j] = removeIgnoredFields("", typ)
		}
	}
	return typsOfTyps
}

// removeIgnoredFields will remove fields we want to ignore from any given types.
func removeIgnoredFields(name string, typ types.Type) types.Type {
	// If the current type is a pointer, call removeIgnoredFields for the type the pointer points to.
	if ptr, ok := typ.(*types.Pointer); ok {
		return types.NewPointer(removeIgnoredFields(name, ptr.Elem()))
	}

	// If this is named, call removeIgnoredFields for the underlying type and pass along the name.
	if named, ok := typ.(*types.Named); ok {
		methods := make([]*types.Func, named.NumMethods())
		for i := 0; i < named.NumMethods(); i++ {
			methods[i] = named.Method(i)
		}
		return types.NewNamed(named.Obj(), removeIgnoredFields(named.Obj().Name(), named.Underlying()), methods)
	}

	// If this is a struct, filter out ignored fields from the struct.
	if strct, ok := typ.(*types.Struct); ok {
		return removeIgnoredFieldsFromStruct(name, strct)
	}

	return typ
}

// removeIgnoredFieldsFromStruct will remove the following fields from structs it encounters:
// - ID/Revision from the Metadata struct.
// - protobuf XXX_ fields.
// - Status fields that are sitting aside a Spec field.
func removeIgnoredFieldsFromStruct(name string, strct *types.Struct) types.Type {
	numFields := strct.NumFields()
	var filteredFields []*types.Var
	var filteredTags []string
	var hasSpec bool

	// Figure out if the field has a spec. If it does, we should ignore any found status fields.
	for i := 0; i < numFields; i++ {
		if strct.Field(i).Name() == "Spec" {
			hasSpec = true
		}
	}

	for i := 0; i < numFields; i++ {
		field := strct.Field(i)
		fieldName := field.Name()

		// Ignore status fields that sit aside spec fields.
		if hasSpec && fieldName == "Status" {
			continue
		}

		// Ignore XXX_ fields, which are proto fields.
		if strings.HasPrefix(fieldName, "XXX_") {
			continue
		}

		// If this is the metadata struct, disregard the ID and Revision fields.
		if strings.HasPrefix(name, "Metadata") && (fieldName == "ID" || fieldName == "Revision") {
			continue
		}

		filteredFields = append(filteredFields, field)
		filteredTags = append(filteredTags, strct.Tag(i))

	}
	if len(filteredFields) != numFields {
		return types.NewStruct(filteredFields, filteredTags)
	}

	return strct
}
