/*
Copyright 2015 Gravitational, Inc.

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

package web

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalFS(t *testing.T) {
	t.Parallel()

	fs, err := NewDebugFileSystem("../../webassets/teleport")
	require.NoError(t, err)
	require.NotNil(t, fs)

	f, err := fs.Open("/index.html")
	require.NoError(t, err)
	bytes, err := io.ReadAll(f)
	require.NoError(t, err)

	html := string(bytes[:])
	require.NoError(t, f.Close())
	require.Equal(t, strings.Contains(html, `<script src="/web/config.js"></script>`), true)
	require.Equal(t, strings.Contains(html, `content="{{ .XCSRF }}"`), true)
}
