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

package azure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeLocation(t *testing.T) {
	require.Equal(t, "eastus", NormalizeLocation("eastus"))
	require.Equal(t, "canadacentral", NormalizeLocation("Canada Central"))
	require.Equal(t, "eastusstage", NormalizeLocation("East US (Stage)"))
	require.Equal(t, "westus2stage", NormalizeLocation("(US) West US 2 (Stage)"))
	require.Equal(t, "uk", NormalizeLocation("United Kingdom"))
	require.Equal(t, "somenewlocation5", NormalizeLocation("Some New Location 5"))
}

func TestGetLocationDisplayName(t *testing.T) {
	require.Equal(t, "Canada Central", GetLocationDisplayName("canadacentral"))
	require.Equal(t, "United Kingdom", GetLocationDisplayName("uk"))
	require.Equal(t, "West US 2 (Stage)", GetLocationDisplayName("West US 2 (Stage)"))
	require.Equal(t, "unknownlocation", GetLocationDisplayName("unknownlocation"))
}
