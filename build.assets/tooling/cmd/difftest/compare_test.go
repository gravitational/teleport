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

package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangedMethods(t *testing.T) {
	head, err := parseMethodMap(filepath.Join("testdata", "ast", "head", "a_simple_test.go"), nil, nil)
	require.NoError(t, err)

	forkPoint, err := parseMethodMap(filepath.Join("testdata", "ast", "fork-point", "a_simple_test.go"), nil, nil)
	require.NoError(t, err)

	r := compare(forkPoint, head)

	assert.True(t, r.HasNew())
	assert.True(t, r.HasChanged())

	assert.Equal(t, r.New, []Method{{Name: "TestFourth", SHA1: "035a07a1e38e5387cd682b2c6b37114d187fa3d2", RefName: "TestFourth"}})
	assert.Equal(t, r.Changed, []Method{{Name: "TestFirst", SHA1: "f045d205e581369b1c7c4148086c838c710f97c8", RefName: "TestFirst"}})
}
