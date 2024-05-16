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
package filename

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
)

func TestParseFilename(t *testing.T) {
	t.Run("WithLeadingPath", func(t *testing.T) {
		info, err := Parse("/some/path/to/file/terraform-provider-teleport-v7.0.0-darwin-amd64-bin.tar.gz")
		require.NoError(t, err)

		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("7.0.0"), info.Version)
		require.Equal(t, "darwin", info.OS)
		require.Equal(t, "amd64", info.Arch)
	})

	t.Run("WithDarwinAmd64", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v13.0.0-darwin-amd64-bin.tar.gz")
		require.NoError(t, err)

		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("13.0.0"), info.Version)
		require.Equal(t, "darwin", info.OS)
		require.Equal(t, "amd64", info.Arch)
	})

	t.Run("WithDarwinArm64", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v13.0.0-darwin-arm64-bin.tar.gz")
		require.NoError(t, err)

		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("13.0.0"), info.Version)
		require.Equal(t, "darwin", info.OS)
		require.Equal(t, "arm64", info.Arch)
	})

	t.Run("WithoutLeadingPath", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v1.2.3-linux-arm-bin.tar.gz")
		require.NoError(t, err)
		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("1.2.3"), info.Version)
		require.Equal(t, "linux", info.OS)
		require.Equal(t, "arm", info.Arch)
	})

	t.Run("RandomJunk", func(t *testing.T) {
		_, err := Parse("blahblahblah")
		require.Error(t, err)
	})

	t.Run("WithPreRelease", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v1.2.3-beta.1-linux-arm-bin.tar.gz")
		require.NoError(t, err)
		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("1.2.3-beta.1"), info.Version)
		require.Equal(t, "linux", info.OS)
		require.Equal(t, "arm", info.Arch)
	})

	t.Run("WithBuild", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v1.2.3+1-linux-arm-bin.tar.gz")
		require.NoError(t, err)
		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("1.2.3+1"), info.Version)
		require.Equal(t, "linux", info.OS)
		require.Equal(t, "arm", info.Arch)
	})

	t.Run("WithPreReleaseAndBuild", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v1.2.3-beta.1+42-linux-arm-bin.tar.gz")
		require.NoError(t, err)
		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("1.2.3-beta.1+42"), info.Version)
		require.Equal(t, "linux", info.OS)
		require.Equal(t, "arm", info.Arch)
	})

	t.Run("UnsupportedOS", func(t *testing.T) {
		_, err := Parse("terraform-provider-teleport-v1.2.3-beos-arm-bin.tar.gz")
		require.Error(t, err)
	})
}

func TestGenerateFilename(t *testing.T) {

	info := Info{
		Type:    "some-plugin",
		Version: *semver.New("1.2.3"),
		OS:      "darwin",
		Arch:    "amd64",
	}
	fn := info.Filename(".banana")
	require.Equal(t, "some-plugin-teleport-v1.2.3-darwin-amd64-bin.banana", fn)
}
