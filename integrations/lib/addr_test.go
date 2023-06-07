// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddrToURL(t *testing.T) {
	url, err := AddrToURL("foo")
	assert.NoError(t, err)
	assert.Equal(t, "https://foo", url.String())

	url, err = AddrToURL("foo:443")
	assert.NoError(t, err)
	assert.Equal(t, "https://foo", url.String())

	url, err = AddrToURL("foo:3080")
	assert.NoError(t, err)
	assert.Equal(t, "https://foo:3080", url.String())
}
