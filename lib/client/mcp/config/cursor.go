package config

import (
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
)

const (
	// cursorProjectDir defines the Cursor project directory, where the MCP
	// configuration is located.
	//
	// https://docs.cursor.com/en/context/mcp#configuration-locations
	cursorProjectDir = ".cursor"
)

// GlobalCursorPath returns the default path for Cursor global MCP configuration.
//
// https://docs.cursor.com/context/mcp#configuration-locations
func GlobalCursorPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return filepath.Join(homeDir, ".cursor", "mcp.json"), nil
}

// LoadConfigFromGlobalCursor loads the Cursor global MCP server configuration.
func LoadConfigFromGlobalCursor() (*FileConfig, error) {
	configPath, err := GlobalCursorPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := LoadConfigFromFile(configPath)
	return config, trace.Wrap(err)
}
