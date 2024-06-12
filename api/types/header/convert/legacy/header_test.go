/*
Copyright 2024 Gravitational, Inc.

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

package legacy

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

func TestFromHeaderMetadata(t *testing.T) {
	expires := time.Now()

	expectedHeader := types.Metadata{
		ID:          12345,
		Name:        "name",
		Expires:     &expires,
		Description: "description",
		Labels:      map[string]string{"label": "value"},
		Revision:    "revision",
	}

	converted := FromHeaderMetadata(header.Metadata{
		Name:        "name",
		Description: "description",
		Labels:      map[string]string{"label": "value"},
		Expires:     expires,
		ID:          12345,
		Revision:    "revision",
	})

	require.Empty(t, cmp.Diff(expectedHeader, converted))
}
