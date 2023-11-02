/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestWindowsDesktopsSorter(t *testing.T) {
	t.Parallel()

	testValsUnordered := []string{"d", "b", "a", "c"}

	makeDesktops := func(testVals []string, testField string) []WindowsDesktop {
		desktops := make([]WindowsDesktop, len(testVals))
		for i := 0; i < len(testVals); i++ {
			testVal := testVals[i]
			var err error
			desktops[i], err = NewWindowsDesktopV3(
				getTestVal(testField == ResourceMetadataName, testVal),
				nil,
				WindowsDesktopSpecV3{
					Addr: getTestVal(testField == ResourceSpecAddr, testVal),
				})

			require.NoError(t, err)
		}
		return desktops
	}

	cases := []struct {
		name      string
		fieldName string
	}{
		{
			name:      "by name",
			fieldName: ResourceMetadataName,
		},
		{
			name:      "by addr",
			fieldName: ResourceSpecAddr,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%s desc", c.name), func(t *testing.T) {
			sortBy := SortBy{Field: c.fieldName, IsDesc: true}
			servers := WindowsDesktops(makeDesktops(testValsUnordered, c.fieldName))
			require.NoError(t, servers.SortByCustom(sortBy))
			targetVals, err := servers.GetFieldVals(c.fieldName)
			require.NoError(t, err)
			require.IsDecreasing(t, targetVals)
		})

		t.Run(fmt.Sprintf("%s asc", c.name), func(t *testing.T) {
			sortBy := SortBy{Field: c.fieldName}
			servers := WindowsDesktops(makeDesktops(testValsUnordered, c.fieldName))
			require.NoError(t, servers.SortByCustom(sortBy))
			targetVals, err := servers.GetFieldVals(c.fieldName)
			require.NoError(t, err)
			require.IsIncreasing(t, targetVals)
		})
	}

	// Test error.
	sortBy := SortBy{Field: "unsupported"}
	desktops := makeDesktops(testValsUnordered, "does-not-matter")
	require.True(t, trace.IsNotImplemented(WindowsDesktops(desktops).SortByCustom(sortBy)))
}

func TestInvalidDesktopName(t *testing.T) {
	_, err := NewWindowsDesktopV3("name-contains.period", nil,
		WindowsDesktopSpecV3{Addr: "desktop.example.com:3389"})
	require.True(t, trace.IsBadParameter(err), "want bad parameter error, got %v", err)
}
