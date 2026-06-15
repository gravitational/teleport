package yamlmod

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAndRender(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple mapping",
			input: "teleport:\n  data_dir: /var/lib/teleport\n",
		},
		{
			name:  "preserves comments",
			input: "# top comment\nteleport:\n  # inline comment\n  data_dir: /var/lib/teleport\n",
		},
		{
			name:  "preserves key ordering",
			input: "z_key: z\na_key: a\nm_key: m\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse([]byte(tt.input))
			require.NoError(t, err)
			require.NotNil(t, doc)

			out, err := Render(doc)
			require.NoError(t, err)
			require.Equal(t, tt.input, string(out))
		})
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := Parse([]byte(":\n  bad:\n    - [unmatched"))
	require.Error(t, err)
}

func TestSet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		path     string
		value    string
		expected string
	}{
		{
			name:     "set existing scalar",
			input:    "teleport:\n  data_dir: /old/path\n",
			path:     "teleport.data_dir",
			value:    "/new/path",
			expected: "teleport:\n  data_dir: /new/path\n",
		},
		{
			name:     "set new key in existing mapping",
			input:    "teleport:\n  data_dir: /var/lib/teleport\n",
			path:     "teleport.auth_token",
			value:    "my-token",
			expected: "teleport:\n  data_dir: /var/lib/teleport\n  auth_token: my-token\n",
		},
		{
			name:     "create intermediate mapping",
			input:    "version: v3\n",
			path:     "teleport.data_dir",
			value:    "/var/lib/teleport",
			expected: "version: v3\nteleport:\n  data_dir: /var/lib/teleport\n",
		},
		{
			name:     "set with array index",
			input:    "app_service:\n  apps:\n    - name: old-app\n      uri: http://localhost:8080\n",
			path:     "app_service.apps[0].name",
			value:    "new-app",
			expected: "app_service:\n  apps:\n    - name: new-app\n      uri: http://localhost:8080\n",
		},
		{
			name:     "preserves comments on sibling keys",
			input:    "teleport:\n  # keep this comment\n  data_dir: /var/lib/teleport\n  auth_token: old\n",
			path:     "teleport.auth_token",
			value:    "new",
			expected: "teleport:\n  # keep this comment\n  data_dir: /var/lib/teleport\n  auth_token: new\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse([]byte(tt.input))
			require.NoError(t, err)

			err = Set(doc, tt.path, tt.value)
			require.NoError(t, err)

			out, err := Render(doc)
			require.NoError(t, err)
			require.Equal(t, tt.expected, string(out))
		})
	}
}

func TestSetBool(t *testing.T) {
	input := "proxy_service:\n  enabled: \"no\"\n"
	expected := "proxy_service:\n  enabled: \"yes\"\n"

	doc, err := Parse([]byte(input))
	require.NoError(t, err)

	err = SetBool(doc, "proxy_service.enabled", true)
	require.NoError(t, err)

	out, err := Render(doc)
	require.NoError(t, err)
	require.Equal(t, expected, string(out))
}

func TestGet(t *testing.T) {
	input := "teleport:\n  data_dir: /var/lib/teleport\n  auth_token: my-token\n"

	doc, err := Parse([]byte(input))
	require.NoError(t, err)

	val, err := Get(doc, "teleport.data_dir")
	require.NoError(t, err)
	require.Equal(t, "/var/lib/teleport", val)

	val, err = Get(doc, "teleport.auth_token")
	require.NoError(t, err)
	require.Equal(t, "my-token", val)

	_, err = Get(doc, "teleport.nonexistent")
	require.Error(t, err)
}

func TestExists(t *testing.T) {
	input := "teleport:\n  data_dir: /var/lib/teleport\n"

	doc, err := Parse([]byte(input))
	require.NoError(t, err)

	require.True(t, Exists(doc, "teleport.data_dir"))
	require.True(t, Exists(doc, "teleport"))
	require.False(t, Exists(doc, "teleport.nonexistent"))
	require.False(t, Exists(doc, "nonexistent"))
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		path     string
		expected string
		wantErr  bool
	}{
		{
			name:     "delete existing key",
			input:    "teleport:\n  data_dir: /var/lib/teleport\n  auth_token: my-token\n",
			path:     "teleport.auth_token",
			expected: "teleport:\n  data_dir: /var/lib/teleport\n",
		},
		{
			name:     "delete top-level key",
			input:    "version: v3\nteleport:\n  data_dir: /var/lib/teleport\n",
			path:     "version",
			expected: "teleport:\n  data_dir: /var/lib/teleport\n",
		},
		{
			name:    "error on non-existent path",
			input:   "teleport:\n  data_dir: /var/lib/teleport\n",
			path:    "teleport.nonexistent",
			wantErr: true,
		},
		{
			name:     "preserves comments on remaining keys",
			input:    "teleport:\n  # keep this\n  data_dir: /var/lib/teleport\n  auth_token: remove-me\n",
			path:     "teleport.auth_token",
			expected: "teleport:\n  # keep this\n  data_dir: /var/lib/teleport\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse([]byte(tt.input))
			require.NoError(t, err)

			err = Delete(doc, tt.path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			out, err := Render(doc)
			require.NoError(t, err)
			require.Equal(t, tt.expected, string(out))
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		dst      string
		key      string
		src      string
		expected string
	}{
		{
			name:     "merge new top-level key",
			dst:      "teleport:\n  data_dir: /var/lib/teleport\n",
			key:      "ssh_service",
			src:      "enabled: \"yes\"\nlisten_addr: 0.0.0.0:3022\n",
			expected: "teleport:\n  data_dir: /var/lib/teleport\nssh_service:\n  enabled: \"yes\"\n  listen_addr: 0.0.0.0:3022\n",
		},
		{
			name:     "no-op if key already exists",
			dst:      "teleport:\n  data_dir: /var/lib/teleport\nssh_service:\n  enabled: \"no\"\n",
			key:      "ssh_service",
			src:      "enabled: \"yes\"\nlisten_addr: 0.0.0.0:3022\n",
			expected: "teleport:\n  data_dir: /var/lib/teleport\nssh_service:\n  enabled: \"no\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse([]byte(tt.dst))
			require.NoError(t, err)

			srcDoc, err := Parse([]byte(tt.src))
			require.NoError(t, err)

			err = Merge(doc, tt.key, srcDoc)
			require.NoError(t, err)

			out, err := Render(doc)
			require.NoError(t, err)
			require.Equal(t, tt.expected, string(out))
		})
	}
}
