package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"
)

// claudeServersKey defines the JSON key used by Claude to store MCP servers
// config.
const claudeServersKey = "mcpServers"

// DefaultClaudeConfigPath returns the default path for the Claude Desktop config.
//
// https://modelcontextprotocol.io/quickstart/user
//
// macOS: ~/Library/Application Support/Claude/claude_desktop_config.json
// Windows: %APPDATA%\Claude\claude_desktop_config.json
func DefaultClaudeConfigPath() (string, error) {
	switch runtime.GOOS {
	case "darwin", "windows":
		// os.UserConfigDir:
		// On Darwin, it returns $HOME/Library/Application Support.
		// On Windows, it returns %AppData%.
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", trace.ConvertSystemError(err)
		}
		return filepath.Join(configDir, "Claude", "claude_desktop_config.json"), nil

	default:
		// TODO(greedy52) there is no official Claude Desktop for linux yet. The
		// unofficial one uses the same path as above.
		return "", trace.NotImplemented("Claude Desktop is not supported on OS %s", runtime.GOOS)
	}
}

// LoadClaudeConfigFromDefaultPath loads the Claude Desktop's config from the
// default path.
func LoadClaudeConfigFromDefaultPath() (*FileConfig, error) {
	configPath, err := DefaultClaudeConfigPath()
	if err != nil {
		return nil, trace.Wrap(err, "finding Claude Desktop config path")
	}
	config, err := LoadConfigFromFile(configPath)
	return config, trace.Wrap(err)
}
