/*
Copyright 2023 Gravitational, Inc.

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

package utils

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenameHeaders(t *testing.T) {
	header := make(http.Header)
	header.Add("aaa", "a1")
	header.Add("aaa", "a2")
	header.Add("bbb", "b1")
	header.Add("ccc", "c1")

	RenameHeader(header, "aaa", "aaaa")
	RenameHeader(header, "bbb", "bbbb")
	RenameHeader(header, "ccc", "ccc")
	require.Equal(t, http.Header{
		"Aaaa": []string{"a1", "a2"},
		"Bbbb": []string{"b1"},
		"Ccc":  []string{"c1"},
	}, header)
}

func TestGetAnyHeader(t *testing.T) {
	header := make(http.Header)
	header.Set("aaa", "a1")
	header.Set("bbb", "b1")

	require.Equal(t, "", GetAnyHeader(header))
	require.Equal(t, "", GetAnyHeader(header, "ccc"))
	require.Equal(t, "a1", GetAnyHeader(header, "aaa"))
	require.Equal(t, "a1", GetAnyHeader(header, "ccc", "aaa"))
	require.Equal(t, "b1", GetAnyHeader(header, "bbb", "aaa"))
}
