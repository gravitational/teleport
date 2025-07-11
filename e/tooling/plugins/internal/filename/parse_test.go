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
		require.Equal(t, "", info.Variant)
	})

	t.Run("WithDarwinAmd64", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v13.0.0-darwin-amd64-bin.tar.gz")
		require.NoError(t, err)

		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("13.0.0"), info.Version)
		require.Equal(t, "darwin", info.OS)
		require.Equal(t, "amd64", info.Arch)
		require.Equal(t, "", info.Variant)
	})

	t.Run("WithDarwinArm64", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v13.0.0-darwin-arm64-bin.tar.gz")
		require.NoError(t, err)

		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("13.0.0"), info.Version)
		require.Equal(t, "darwin", info.OS)
		require.Equal(t, "arm64", info.Arch)
		require.Equal(t, "", info.Variant)
	})

	t.Run("WithVariant", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleportmwi-v13.0.0-darwin-amd64-bin.tar.gz")
		require.NoError(t, err)

		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("13.0.0"), info.Version)
		require.Equal(t, "darwin", info.OS)
		require.Equal(t, "amd64", info.Arch)
		require.Equal(t, "mwi", info.Variant)
	})

	t.Run("WithoutLeadingPath", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v1.2.3-linux-arm-bin.tar.gz")
		require.NoError(t, err)
		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("1.2.3"), info.Version)
		require.Equal(t, "linux", info.OS)
		require.Equal(t, "arm", info.Arch)
		require.Equal(t, "", info.Variant)
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
		require.Equal(t, "", info.Variant)
	})

	t.Run("WithBuild", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v1.2.3+1-linux-arm-bin.tar.gz")
		require.NoError(t, err)
		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("1.2.3+1"), info.Version)
		require.Equal(t, "linux", info.OS)
		require.Equal(t, "arm", info.Arch)
		require.Equal(t, "", info.Variant)
	})

	t.Run("WithPreReleaseAndBuild", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleport-v1.2.3-beta.1+42-linux-arm-bin.tar.gz")
		require.NoError(t, err)
		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("1.2.3-beta.1+42"), info.Version)
		require.Equal(t, "linux", info.OS)
		require.Equal(t, "arm", info.Arch)
		require.Equal(t, "", info.Variant)
	})

	// This case ensures the regex handles versions with strings within them.
	t.Run("WithUnusualVariantAndVersion", func(t *testing.T) {
		info, err := Parse("terraform-provider-teleportvariant-v1.23.0-very-new-feature-linux-amd64-bin.tar.gz")
		require.NoError(t, err)
		require.Equal(t, "terraform-provider", info.Type)
		require.Equal(t, *semver.New("1.23.0-very-new-feature"), info.Version)
		require.Equal(t, "linux", info.OS)
		require.Equal(t, "amd64", info.Arch)
		require.Equal(t, "variant", info.Variant)
	})

	t.Run("UnsupportedOS", func(t *testing.T) {
		_, err := Parse("terraform-provider-teleport-v1.2.3-beos-arm-bin.tar.gz")
		require.Error(t, err)
	})
}

func TestGenerateFilename(t *testing.T) {
	t.Run("no variant", func(t *testing.T) {
		info := Info{
			Type:    "some-plugin",
			Version: *semver.New("1.2.3"),
			OS:      "darwin",
			Arch:    "amd64",
		}
		fn := info.Filename(".banana")
		require.Equal(t, "some-plugin-teleport-v1.2.3-darwin-amd64-bin.banana", fn)
	})
	t.Run("with variant", func(t *testing.T) {
		info := Info{
			Type:    "some-plugin",
			Version: *semver.New("1.2.3"),
			OS:      "darwin",
			Arch:    "amd64",
			Variant: "mwi",
		}
		fn := info.Filename(".banana")
		require.Equal(t, "some-plugin-teleportmwi-v1.2.3-darwin-amd64-bin.banana", fn)
	})
}
