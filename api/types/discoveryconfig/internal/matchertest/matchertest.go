/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package matchertest provides reflection-based test helpers that enumerate
// the matcher families of a discovery config spec carrying installer params,
// so synthetic sanitization coverage cannot silently drift when a new family
// is added. It takes the spec as an untyped pointer instead of importing the
// discoveryconfig package so that package's own in-package tests can use it
// without an import cycle.
package matchertest

import (
	"reflect"

	"github.com/gravitational/teleport/api/types"
)

// SentinelJoinToken marks the installer params planted by
// PopulateSentinelInstallerParams.
const SentinelJoinToken = "sentinel"

var installerParamsType = reflect.TypeOf((*types.InstallerParams)(nil))

// PopulateSentinelInstallerParams plants one element holding sentinel
// installer params in every matcher family of the spec, and returns the
// number of families populated. specPtr must be a pointer to a discovery
// config Spec struct.
func PopulateSentinelInstallerParams(specPtr any) int {
	specVal := reflect.ValueOf(specPtr).Elem()
	specType := specVal.Type()
	populated := 0
	for i := range specType.NumField() {
		field := specType.Field(i)
		params, ok := installerParamsField(field.Type)
		if !ok {
			continue
		}
		entry := reflect.New(field.Type.Elem()).Elem()
		entry.FieldByIndex(params.Index).Set(reflect.ValueOf(&types.InstallerParams{JoinToken: SentinelJoinToken}))
		specVal.Field(i).Set(reflect.Append(reflect.MakeSlice(field.Type, 0, 1), entry))
		populated++
	}
	return populated
}

// FamiliesWithInstallerParams returns the name of the matcher family for each
// spec element still carrying non-nil installer params, one entry per element.
func FamiliesWithInstallerParams(specPtr any) []string {
	var found []string
	specVal := reflect.ValueOf(specPtr).Elem()
	specType := specVal.Type()
	for i := range specType.NumField() {
		field := specType.Field(i)
		params, ok := installerParamsField(field.Type)
		if !ok {
			continue
		}
		slice := specVal.Field(i)
		for j := range slice.Len() {
			if !slice.Index(j).FieldByIndex(params.Index).IsNil() {
				found = append(found, field.Name)
			}
		}
	}
	return found
}

// installerParamsField reports whether fieldType is a matcher family: a slice
// of structs whose element type carries a *types.InstallerParams field named
// Params.
func installerParamsField(fieldType reflect.Type) (reflect.StructField, bool) {
	if fieldType.Kind() != reflect.Slice || fieldType.Elem().Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}
	params, ok := fieldType.Elem().FieldByName("Params")
	if !ok || params.Type != installerParamsType {
		return reflect.StructField{}, false
	}
	return params, true
}
