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
package release

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	// should err when no TLS config is passed in
	_, err := NewClient(ClientConfig{})
	assert.NotNil(t, err)

	// should not err when TLS config is passed in
	_, err = NewClient(ClientConfig{
		TLSConfig: &tls.Config{},
	})
	assert.Nil(t, err)
}

func TestByteCount(t *testing.T) {
	tt := []struct {
		name     string
		size     int64
		expected string
	}{
		{
			name:     "1 byte",
			size:     1,
			expected: "1 B",
		},
		{
			name:     "2 byte2",
			size:     2,
			expected: "2 B",
		},
		{
			name:     "1kb",
			size:     1000,
			expected: "1.0 kB",
		},
		{
			name:     "1mb",
			size:     1000_000,
			expected: "1.0 MB",
		},
		{
			name:     "1gb",
			size:     1000_000_000,
			expected: "1.0 GB",
		},
		{
			name:     "1tb",
			size:     1000_000_000_000,
			expected: "1.0 TB",
		},
		{
			name:     "1.6 kb",
			size:     1600,
			expected: "1.6 kB",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, byteCount(tc.size), tc.expected)
		})
	}
}
