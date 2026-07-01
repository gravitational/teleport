package recon

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

func TestRenderScriptShellcheck(t *testing.T) {
	script := RenderScript()
	require.NotEmpty(t, script)
	require.Contains(t, script, "#!/bin/bash")

	// Write to temp file and run shellcheck if available
	if _, err := exec.LookPath("shellcheck"); err != nil {
		t.Skip("shellcheck not installed")
	}
	tmp := filepath.Join(t.TempDir(), "probe.sh")
	require.NoError(t, os.WriteFile(tmp, []byte(script), 0755))
	out, err := exec.Command("shellcheck", "-s", "bash", tmp).CombinedOutput()
	require.NoError(t, err, "shellcheck failed:\n%s", string(out))
}

func TestRenderScriptReadOnly(t *testing.T) {
	script := RenderScript()
	// Must not contain any write operations
	require.NotContains(t, script, "rm ")
	require.NotContains(t, script, "mv ")
	require.NotContains(t, script, "tee ")
	require.NotContains(t, script, "> /")
	require.NotContains(t, script, ">> /")
}

func TestParseOutputLinux(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "probe_output_linux.txt"))
	require.NoError(t, err)
	result, err := ParseOutput(string(data))
	require.NoError(t, err)
	require.True(t, result.Reachable)
	require.Equal(t, "Linux", result.OS)
	require.True(t, result.HasSystemd)
	require.True(t, result.HasTeleportUpdate)
	require.Equal(t, "/etc/teleport.yaml", result.ConfigPath)
	require.True(t, result.RootPath)
	require.Equal(t, "token", result.JoinMethod)
	require.Contains(t, result.Services, "ssh_service")
	require.Equal(t, classify.InstallSystemd, result.InstallKind)
}

func TestParseOutputNoRoot(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "probe_output_noroot.txt"))
	require.NoError(t, err)
	result, err := ParseOutput(string(data))
	require.NoError(t, err)
	require.False(t, result.RootPath)
}
