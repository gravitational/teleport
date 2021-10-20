/*
Copyright 2021 Gravitational, Inc.
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

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func init() {
	utils.InitLogger(utils.LoggingForCLI, log.DebugLevel)
}

func main() {
	var buildFlags []string
	if len(os.Args) > 1 {
		log.Infof("Using buildFlags: %v", buildFlags)
		buildFlags = os.Args[1:]
	}

	// the api module import path should only be updated on releases
	newVersion := api.Version
	if isPreRelease(newVersion) {
		exitWithMessage("the current API version (%v) is not a release, continue without updating", newVersion)
	}

	// get the current api module import path
	currentModPath, err := getModImportPath("./api")
	if err != nil {
		exitWithError(trace.Wrap(err, "failed to get current mod path"))
	}

	// get the new api module import path, and exit if the path hasn't changed
	newPath := getNewModImportPath(currentModPath, newVersion)
	if currentModPath == newPath {
		exitWithMessage("the api module path has not changed, continue without updating")
	}

	// update go files within the teleport/api and teleport modules to use the new import path
	log.Info("Updating teleport/api module...")
	if err := updateGoModule("./api", currentModPath, newPath, newVersion, buildFlags); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport/api module"))
	}
	log.Info("Updating teleport module...")
	if err := updateGoModule("./", currentModPath, newPath, newVersion, buildFlags); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport module"))
	}

	// Update .proto files in teleport/api to use the new import path
	log.Info("Updating .proto files...")
	if err := updateProtoFiles("./api", currentModPath, newPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport mod file"))
	}
}

// updateGoModule updates instances of the currentPath with the newPath in the given go module.
func updateGoModule(modulePath, currentPath, newPath, newVersion string, buildFlags []string) error {
	log.Info("    Updating go files...")
	if err := updateGoPkgs(modulePath, currentPath, newPath, buildFlags); err != nil {
		return trace.Wrap(err, "failed to update mod file for module")
	}

	log.Info("    Updating go.mod...")
	if err := updateGoModFile(modulePath, currentPath, newPath, newVersion); err != nil {
		return trace.Wrap(err, "failed to update mod file for module")
	}

	return nil
}

// updateGoPkgs updates instances of the currentPath with the newPath in go pkgs in the given module.
func updateGoPkgs(rootDir, currentPath, newPath string, buildFlags []string) error {
	mode := packages.NeedTypes | packages.NeedSyntax
	cfg := &packages.Config{Mode: mode, Tests: true, Dir: rootDir, BuildFlags: buildFlags}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(pkgs)

	var errs []error
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if err = updateGoImports(pkg, currentPath, newPath); err != nil {
			errs = append(errs, err)
			return false
		}
		return true
	}, nil)
	return trace.NewAggregate(errs...)
}

// updateGoImports updates instances of the currentPath with the newPath in the given package.
func updateGoImports(p *packages.Package, currentPath, newPath string) error {
	for _, syn := range p.Syntax {
		var rewritten bool
		for _, i := range syn.Imports {
			imp := strings.Replace(i.Path.Value, "\"", "", 2)
			if strings.HasPrefix(imp, currentPath) && !strings.HasPrefix(imp, newPath) {
				newImp := strings.Replace(imp, currentPath, newPath, 1)
				if astutil.RewriteImport(p.Fset, syn, imp, newImp) {
					rewritten = true
				}
			}
		}
		if !rewritten {
			continue
		}

		goFileName := p.Fset.File(syn.Pos()).Name()
		f, err := os.OpenFile(goFileName, os.O_RDWR, 0660)
		if err != nil {
			return trace.Wrap(err, "could not open go file %v", goFileName)
		}
		defer f.Close()

		if err = format.Node(f, p.Fset, syn); err != nil {
			return trace.Wrap(err, "could not rewrite go file %v", goFileName)
		}
	}

	return nil
}

// updateGoModFile updates instances of oldPath to newPath in a go.mod file.
// The modFile is updated in place by updating the syntax fields directly.
func updateGoModFile(dir, oldPath, newPath, newVersion string) error {
	modFile, err := getModFile(dir)
	if err != nil {
		return trace.Wrap(err, "failed to get mod file")
	}
	// Update mod name if needed
	if modFile.Module.Syntax.Token[1] == oldPath {
		modFile.Module.Syntax.Token[1] = newPath
	}
	// Update require statements in place if needed
	for _, r := range modFile.Require {
		if r.Mod.Path == oldPath {
			// Update path and version of require statement.
			if r.Syntax.InBlock {
				r.Syntax.Token[0], r.Syntax.Token[1] = newPath, "v"+newVersion
			} else {
				// First token in the line is "require", skip to second and third indices
				r.Syntax.Token[1], r.Syntax.Token[2] = newPath, "v"+newVersion
			}
		}
	}
	// Update replace statements in place if needed
	for _, r := range modFile.Replace {
		if r.Old.Path == oldPath {
			// Update path of replace statement.
			if r.Syntax.InBlock {
				r.Syntax.Token[0] = newPath
				if r.Old.Version != "" {
					r.Syntax.Token[1] = "v" + newVersion
				}
			} else {
				// First token in the line is "replace", skip to second index
				r.Syntax.Token[1] = newPath
				if r.Old.Version != "" {
					r.Syntax.Token[2] = "v" + newVersion
				}
			}
		}
	}

	// Format and save mod file
	bytes, err := modFile.Format()
	if err != nil {
		return trace.Wrap(err, "failed to format go.mod file with new import path")
	}
	info, err := os.Stat(filepath.Join(dir, "go.mod"))
	if err != nil {
		return trace.Wrap(err, "failed to go.mod file info")
	}
	if err = os.WriteFile(filepath.Join(dir, "go.mod"), bytes, info.Mode().Perm()); err != nil {
		return trace.Wrap(err, "failed to rewrite go.mod file")
	}
	return nil
}

// updateProtoFiles updates instances of the currentPath with
// the newPath in .proto files within the given directory
func updateProtoFiles(rootDir, currentPath, newPath string) error {
	return filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if strings.HasSuffix(d.Name(), ".proto") {
			data, err := os.ReadFile(path)
			if err != nil {
				return trace.Wrap(err)
			}
			data = bytes.ReplaceAll(data, []byte(currentPath), []byte(newPath))
			if err := os.WriteFile(path, data, d.Type().Perm()); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
}

// getModImportPath gets the module's currently set path/name
func getModImportPath(dir string) (string, error) {
	modFile, err := getModFile(dir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if modFile.Module.Mod.Path == "" {
		return "", trace.NotFound("could not find mod path for %v", dir)
	}
	return modFile.Module.Mod.Path, nil
}

// getModFile returns an AST of the given go.mod file
func getModFile(dir string) (*modfile.File, error) {
	modPath := filepath.Join(dir, "go.mod")
	bts, err := os.ReadFile(modPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f, err := modfile.Parse(modPath, bts, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return f, nil
}

// getNewModImportPath gets the new import path given a go module import path and the updated version
func getNewModImportPath(oldPath, newVersion string) string {
	// get the new major version suffix - e.g "" for v0/v1 or "/vX" for vX where X >= 2
	var majVerSuffix string
	if ver := semver.New(newVersion); ver.Major >= 2 {
		majVerSuffix = fmt.Sprintf("/v%d", ver.Major)
	}

	// get the new mod path by replacing the current mod path with the new major version suffix
	newPath := oldPath + majVerSuffix
	if suffixIndex := strings.Index(oldPath, "/v"); suffixIndex != -1 {
		newPath = oldPath[:suffixIndex] + majVerSuffix
	}
	return newPath
}

// returns whether the current api version is a pre-release, e.g "v7.0.0-beta"
func isPreRelease(version string) bool {
	return semver.New(version).PreRelease != ""
}

func exitWithError(err error) {
	if err != nil {
		log.WithError(err).Error()
	}
	os.Exit(1)
}

func exitWithMessage(format string, args ...interface{}) {
	log.Infof(format, args...)
	os.Exit(1)
}
