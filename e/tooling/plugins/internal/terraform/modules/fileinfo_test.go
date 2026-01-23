package modules

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
)

func TestParseFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		artifact string
		wantInfo FileInfo
		wantErr  string
	}{
		{
			desc:     "release tag",
			artifact: "/tmp/artifacts/terraform-module_teleport_discovery_aws_v18.99.0.tar.gz",
			wantInfo: FileInfo{
				Path:      "/tmp/artifacts/terraform-module_teleport_discovery_aws_v18.99.0.tar.gz",
				Namespace: "teleport",
				Name:      "discovery",
				System:    "aws",
				Version: semver.Version{
					Major:      18,
					Minor:      99,
					Patch:      0,
					PreRelease: "",
				},
			},
		},
		{
			desc:     "dev tag",
			artifact: "/tmp/artifacts/terraform-module_teleport_discovery_azurerm_v19.0.0-dev.llama.1.tar.gz",
			wantInfo: FileInfo{
				Path:      "/tmp/artifacts/terraform-module_teleport_discovery_azurerm_v19.0.0-dev.llama.1.tar.gz",
				Namespace: "teleport",
				Name:      "discovery",
				System:    "azurerm",
				Version: semver.Version{
					Major:      19,
					Minor:      0,
					Patch:      0,
					PreRelease: "dev.llama.1",
				},
			},
		},
		{
			desc:     "provider artifact",
			artifact: "/tmp/artifacts/terraform-provider-teleport-v19.0.0-dev.noahmwitf.4-darwin-amd64-bin.tar.gz",
			wantErr:  "does not match required pattern",
		},
		{
			desc:     "mwi provider artifact",
			artifact: "/tmp/artifacts/terraform-provider-teleportmwi-v19.0.0-dev.noahmwitf.4-darwin-amd64-bin.tar.gz",
			wantErr:  "does not match required pattern",
		},
		{
			desc:     "empty namespace is rejected",
			artifact: "/tmp/artifacts/terraform-module__discovery_aws_v18.99.0.tar.gz",
			wantErr:  "does not match required pattern",
		},
		{
			desc:     "empty name is rejected",
			artifact: "/tmp/artifacts/terraform-module_teleport__aws_v18.99.0.tar.gz",
			wantErr:  "does not match required pattern",
		},
		{
			desc:     "empty system is rejected",
			artifact: "/tmp/artifacts/terraform-module_teleport_discovery__v18.99.0.tar.gz",
			wantErr:  "does not match required pattern",
		},
		{
			desc:     "empty version is rejected",
			artifact: "/tmp/artifacts/terraform-module_teleport_discovery__v18.99.0.tar.gz",
			wantErr:  "does not match required pattern",
		},
		{
			desc:     "must end in .tar.gz",
			artifact: "/tmp/artifacts/terraform-module_teleport_discovery_aws_v18.99.0xtarxgz",
			wantErr:  "does not match required pattern",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			info, err := ParseFilePath(test.artifact)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, info)
			require.Equal(t, test.wantInfo, *info)
		})
	}
}

func TestIsModuleTarball(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		artifact string
		want     bool
	}{
		{
			desc:     "release tag",
			artifact: "/tmp/artifacts/terraform-module_teleport_discovery_aws_v18.99.0.tar.gz",
			want:     true,
		},
		{
			desc:     "dev tag",
			artifact: "/tmp/artifacts/terraform-module_teleport_discovery_aws_v19.0.0-dev.llama.1.tar.gz",
			want:     true,
		},
		{
			desc:     "provider artifact",
			artifact: "/tmp/artifacts/terraform-provider-teleport-v19.0.0-dev.noahmwitf.4-darwin-amd64-bin.tar.gz",
			want:     false,
		},
		{
			desc:     "mwi provider artifact",
			artifact: "/tmp/artifacts/terraform-provider-teleportmwi-v19.0.0-dev.noahmwitf.4-darwin-amd64-bin.tar.gz",
			want:     false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := IsModuleTarball(test.artifact)
			require.Equal(t, test.want, got)
		})
	}
}
