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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRun exercises code generation end-to-end. It invokes the generator
// with the teleport-kube-agent chart and compares the produced Go source to
// the code generated with go generate. This serves to catch changes to the
// helm chart that aren't reflected in the committed generated code.
func TestRun(t *testing.T) {
	const (
		chartPath     = "../../../../../../../examples/chart/teleport-kube-agent"
		valuesPath    = "../../testdata/values.yaml"
		generatedPath = "../../zz_generated.go"
	)

	outPath := filepath.Join(t.TempDir(), "out.go")
	err := run([]string{
		"-chart", chartPath,
		"-values", valuesPath,
		"-out", outPath,
	})
	require.NoError(t, err)

	got, err := os.ReadFile(outPath)
	require.NoError(t, err)

	want, err := os.ReadFile(generatedPath)
	require.NoError(t, err)

	require.Equal(t, string(got), string(want))
}
