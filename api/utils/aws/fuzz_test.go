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

package aws

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseRDSEndpoint(f *testing.F) {
	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseRDSEndpoint(endpoint)
		})
	})
}

func FuzzParseRedshiftEndpoint(f *testing.F) {
	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _, _ = ParseRedshiftEndpoint(endpoint)
		})
	})
}

func FuzzParseElastiCacheEndpoint(f *testing.F) {
	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseElastiCacheEndpoint(endpoint)
		})
	})
}

func FuzzParseDynamoDBEndpoint(f *testing.F) {
	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseDynamoDBEndpoint(endpoint)
		})
	})
}

func FuzzParseOpensearchEndpoint(f *testing.F) {
	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			_, _ = ParseOpensearchEndpoint(endpoint)
		})
	})
}
