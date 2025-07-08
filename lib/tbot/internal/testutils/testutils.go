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

// Package testutils contains commonly used helpers for testing bot services.
//
// Note: we do not import the testing or testify packages to avoid accidentally
// bringing these dependencies into our production binaries.
package testutils

import (
	"bytes"
	"context"
	"reflect"
	"strings"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

// TestingT is a subset of *testing.T's interface. It is intentionally *NOT*
// compatible with testify's require and assert packages to avoid accidentally
// bringing those packages into production code. See: TestNotTestifyCompatible.
type TestingT interface {
	Cleanup(fn func())
	Context() context.Context
	Fatalf(format string, args ...any)
	Helper()
	Name() string
}

// Pointer returns a pointer to v. It's useful in expressions where it's
// impossible to take a pointer of a literal value.
//
// Example: &"hi" doesn't work but Pointer("hi") does.
func Pointer[T any](v T) *T {
	return &v
}

// Subtester extends TestingT with the Run method for subtests.
type Subtester[T TestingT] interface {
	TestingT

	Run(name string, fn func(T)) bool
}

// TestYAML marshals the given test structs to YAML, compares them to golden
// files, and then unmarshals them to check nothing is lost in the round-trip.
func TestYAML[T TestingT, CaseT any](t Subtester[T], tests []TestYAMLCase[CaseT]) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t T) {
			b := bytes.NewBuffer(nil)
			encoder := yaml.NewEncoder(b)
			encoder.SetIndent(2)

			if err := encoder.Encode(&tt.In); err != nil {
				t.Fatalf("failed to encode: %v", err)
			}

			if golden.ShouldSet() {
				golden.Set(t, b.Bytes())
			}

			if diff := cmp.Diff(
				string(golden.Get(t)),
				b.String(),
			); diff != "" {
				t.Fatalf("results of marshal did not match golden file, rerun tests with GOLDEN_UPDATE=1\n\n%s", diff)
			}

			// Now test unmarshaling to see if we get the same object back.
			var unmarshaled CaseT
			decoder := yaml.NewDecoder(b)
			if err := decoder.Decode(&unmarshaled); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}

			if diff := cmp.Diff(
				tt.In,
				unmarshaled,
				exportAll,
			); diff != "" {
				t.Fatalf("unmarshaling did not result in same object as input\n\n%s", diff)
			}
		})
	}
}

// TestYAMLCase is a test case for TestYAML.
type TestYAMLCase[CaseT any] struct {
	// Name of this case.
	Name string

	// In is the input struct that will be marshaled.
	In CaseT
}

// TestCheckAndSetDefaults tests the CheckAndSetDefaults method.
func TestCheckAndSetDefaults[T TestingT, CaseT CheckAndSetDefaulter](t Subtester[T], tests []TestCheckAndSetDefaultsCase[CaseT]) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.Name, func(t T) {
			got := tt.In()
			err := got.CheckAndSetDefaults()
			if tt.WantErr != "" {
				if !strings.Contains(err.Error(), tt.WantErr) {
					t.Fatalf("error %v does not contain %q", err, tt.WantErr)
				}
				return
			}

			want := tt.Want
			if want == nil {
				want = tt.In()
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(
				want,
				got,
				exportAll,
			); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

type CheckAndSetDefaulter interface {
	CheckAndSetDefaults() error
}

// TestCheckAndSetDefaultsCase is a case for TestCheckAndSetDefaults.
type TestCheckAndSetDefaultsCase[T CheckAndSetDefaulter] struct {
	Name string

	// In returns the input struct.
	In func() T

	// Want specifies the desired state of the checkAndSetDefaulter after
	// check and set defaults has been run. If Want is nil, the Output is
	// compared to its initial state.
	Want CheckAndSetDefaulter

	// WantErr is the error that is expected from calling CheckAndSetDefault.
	WantErr string
}

// exportAll allows go-cmp to compare all fields (even unexported ones).
var exportAll = cmp.Exporter(func(reflect.Type) bool { return true })
