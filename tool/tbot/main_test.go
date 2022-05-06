package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/tool/tbot/config"
	"github.com/stretchr/testify/require"
)

func Test_onConfigure(t *testing.T) {
	t.Parallel()

	cfg := &config.BotConfig{
		AuthServer: "foo:bar",
	}
	// TODO: It would be nice to pull this out into a golden file
	expect := "debug: false\nauth_server: foo:bar\ncertificate_ttl: 0s\nrenewal_interval: 0s\noneshot: false\n"

	t.Run("file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "config.yaml")
		err := onConfigure(&config.BotConfig{}, path)
		require.NoError(t, err)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, expect, string(data))
	})

	t.Run("stdout", func(t *testing.T) {
		t.Parallel()

		stdout := new(bytes.Buffer)
		err := onConfigure(stdout, "")
		require.NoError(t, err)
		require.Equal(t, expect, stdout.String())
	})
}
