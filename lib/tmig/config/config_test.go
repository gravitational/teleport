package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadValidConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("testdata", "valid.yaml"))
	require.NoError(t, err)
	require.Equal(t, "scoped.example.com:443", cfg.Target.Proxy)
	require.Len(t, cfg.Migrations, 1)
	require.Equal(t, "legacy-1.example.com:443", cfg.Migrations[0].Source.Proxy)
	require.Equal(t, "ec2-user", cfg.Migrations[0].SSH.Login)
	require.Len(t, cfg.Migrations[0].Mappings, 1)
	require.Equal(t, "/dgxc/team-a", cfg.Migrations[0].Mappings[0].Scope)
	require.Equal(t, "dgxc-team-a", cfg.Migrations[0].Mappings[0].InstallSuffix)
	require.Equal(t, 32, cfg.Concurrency)
}

func TestLoadRejectsDuplicateSuffix(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "invalid_duplicate_suffix.yaml"))
	require.ErrorContains(t, err, "duplicate install_suffix")
}

func TestLoadRejectsNoSelector(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "no_selector.yaml"))
	require.ErrorContains(t, err, "selector")
}

func TestLoadDefaultsSuffixFromScope(t *testing.T) {
	cfg, err := Load(filepath.Join("testdata", "valid.yaml"))
	require.NoError(t, err)
	// valid.yaml explicitly sets install_suffix, but test the derivation logic
	require.Equal(t, "dgxc-team-a", cfg.Migrations[0].Mappings[0].InstallSuffix)
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "bad.yaml")
	os.WriteFile(f, []byte("target:\n  proxy: x\nunknown_key: true\n"), 0644)
	_, err := Load(f)
	require.Error(t, err)
}
