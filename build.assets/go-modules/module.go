package modules

import (
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"golang.org/x/mod/modfile"
)

// GetImportPath gets the module's import path from its go.mod file
func GetImportPath(dir string) (string, error) {
	modPath := filepath.Join(dir, "go.mod")
	bts, err := os.ReadFile(modPath)
	if err != nil {
		return "", trace.Wrap(err)
	}
	modFile, err := modfile.Parse(modPath, bts, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if modFile.Module.Mod.Path == "" {
		return "", trace.NotFound("could not find mod path for %v", dir)
	}
	return modFile.Module.Mod.Path, nil
}
