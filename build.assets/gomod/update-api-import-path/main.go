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

// Command update-api-import-path updates the api import path to
// incorporate the version set in /api/version.go. If the major
// version hasn't changed or the version is a prelease, no change
// is made. Otherwise, all go.mod files, .go files, and .proto files
// are updated to use the new api import path as needed.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/build.assets/gomod"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func init() {
	utils.InitLogger(utils.LoggingForCLI, log.DebugLevel)
}

// This script should only be run through the make target `make update-api-module-path`
// since it relies on relative paths to the /api and root directories.
func main() {
	var buildFlags []string
	if len(os.Args) > 1 {
		log.Infof("Using buildFlags: %v", buildFlags)
		buildFlags = os.Args[1:]
	}

	// the api module import path should only be updated on releases
	newVersion := semver.New(api.Version)
	if newVersion.PreRelease != "" {
		exitWithMessage("the current API version (%v) is not a release, continue without updating", newVersion)
	}

	// get the current api module import path
	currentModPath, err := gomod.GetImportPath("./api")
	if err != nil {
		exitWithError(trace.Wrap(err, "failed to get current mod path"), nil)
	}

	// get the new api module import path, and exit if the path hasn't changed
	newPath := getNewModImportPath(currentModPath, newVersion)
	if currentModPath == newPath {
		exitWithMessage("the api module path has not changed, continue without updating")
	}

	rollBackFuncs := []rollBackFunc{}
	addRollBack := func(r rollBackFunc) { rollBackFuncs = append(rollBackFuncs, r) }

	// update go files within the teleport/api and teleport modules to use the new import path
	log.Info("Updating teleport/api module...")
	if err := updateGoModule("./api", currentModPath, newPath, newVersion.String(), buildFlags, addRollBack); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport/api module"), rollBackFuncs)
	}
	log.Info("Updating teleport module...")
	if err := updateGoModule("./", currentModPath, newPath, newVersion.String(), buildFlags, addRollBack); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport module"), rollBackFuncs)
	}

	// Update .proto files in teleport/api to use the new import path
	log.Info("Updating .proto files...")
	if err := updateProtoFiles("./api", currentModPath, newPath, addRollBack); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport mod file"), rollBackFuncs)
	}
}

// updateGoModule updates instances of the currentPath with the newPath in the given go module.
func updateGoModule(modulePath, currentPath, newPath, newVersion string, buildFlags []string, addRollBack addRollBackFunc) error {
	log.Info("    Updating go files...")
	if err := updateGoPkgs(modulePath, currentPath, newPath, buildFlags, addRollBack); err != nil {
		return trace.Wrap(err, "failed to update mod file for module")
	}

	log.Info("    Updating go.mod...")
	if err := updateGoModFile(modulePath, currentPath, newPath, newVersion, addRollBack); err != nil {
		return trace.Wrap(err, "failed to update mod file for module")
	}

	return nil
}

// updateGoPkgs updates instances of the currentPath with the newPath in go pkgs in the given module.
func updateGoPkgs(rootDir, currentPath, newPath string, buildFlags []string, addRollBack addRollBackFunc) error {
	mode := packages.NeedTypes | packages.NeedSyntax
	cfg := &packages.Config{Mode: mode, Tests: true, Dir: rootDir, BuildFlags: buildFlags}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return trace.Wrap(err)
	}

	var errs []error
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if err = updateGoImports(pkg, currentPath, newPath, addRollBack); err != nil {
			errs = append(errs, err)
			return false
		}
		return true
	}, nil)
	return trace.NewAggregate(errs...)
}

// updateGoImports updates instances of the currentPath with the newPath in the given package.
func updateGoImports(p *packages.Package, currentPath, newPath string, addRollBack addRollBackFunc) error {
	for _, syn := range p.Syntax {
		var rewritten bool
		for _, i := range syn.Imports {
			imp := strings.Replace(i.Path.Value, "\"", "", 2)
			// Replace all instances of the current path with the new path, but prevent this from happening multiple times in edge cases
			if strings.HasPrefix(imp, currentPath) && (!strings.HasPrefix(newPath, currentPath) || !strings.HasPrefix(imp, newPath)) {
				newImp := strings.Replace(imp, currentPath, newPath, 1)
				if astutil.RewriteImport(p.Fset, syn, imp, newImp) {
					rewritten = true
				}
			}

		}
		if !rewritten {
			continue
		}

		goFilePath := p.Fset.File(syn.Pos()).Name()
		data, err := os.ReadFile(goFilePath)
		if err != nil {
			return trace.Wrap(err, "could not read go file %v", goFilePath)
		}

		// Format updated go file and write to disk
		updatedData := bytes.NewBuffer([]byte{})
		if err = format.Node(updatedData, p.Fset, syn); err != nil {
			return trace.Wrap(err, "could not rewrite go file %v", goFilePath)
		}
		info, err := os.Stat(goFilePath)
		if err != nil {
			return trace.Wrap(err, "failed to get go file info")
		}
		if err = os.WriteFile(goFilePath, updatedData.Bytes(), info.Mode().Perm()); err != nil {
			return trace.Wrap(err, "failed to rewrite go file")
		}

		addRollBack(func() error {
			err := os.WriteFile(goFilePath, data, info.Mode().Perm())
			return trace.Wrap(err, "failed to rollback changes to go file %v", goFilePath)
		})
	}

	return nil
}

// updateGoModFile updates instances of oldPath to newPath in a go.mod file.
// The modFile is updated in place by updating the syntax fields directly.
func updateGoModFile(dir, oldPath, newPath, newVersion string, addRollBackFunc addRollBackFunc) error {
	modFilePath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(modFilePath)
	if err != nil {
		return trace.Wrap(err)
	}
	modFile, err := modfile.Parse(modFilePath, data, nil)
	if err != nil {
		return trace.Wrap(err)
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

	// Format updated go mod file and write to disk
	updatedData, err := modFile.Format()
	if err != nil {
		return trace.Wrap(err, "failed to format go.mod file with new import path")
	}
	info, err := os.Stat(modFilePath)
	if err != nil {
		return trace.Wrap(err, "failed to get go.mod file info")
	}
	if err = os.WriteFile(modFilePath, updatedData, info.Mode().Perm()); err != nil {
		return trace.Wrap(err, "failed to rewrite go.mod file")
	}

	addRollBackFunc(func() error {
		err := os.WriteFile(modFilePath, data, info.Mode().Perm())
		return trace.Wrap(err, "failed to rollback changes to go mod file %v", modFilePath)
	})

	return nil
}

// updateProtoFiles updates gogoproto cast types and custom types in .proto files
// within the given directory to use the new api import path.
func updateProtoFiles(rootDir, currentPath, newPath string, addRollBackFunc addRollBackFunc) error {
	return filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if strings.HasSuffix(d.Name(), ".proto") {
			data, err := os.ReadFile(path)
			if err != nil {
				return trace.Wrap(err)
			}

			// Replace all instances of the api import path in gogoproto casttypes with the new import path
			currentCastTypes := fmt.Sprintf(`(gogoproto.casttype) = "%v`, currentPath)
			newCastTypes := fmt.Sprintf(`(gogoproto.casttype) = "%v`, newPath)
			updatedData := bytes.ReplaceAll(data, []byte(currentCastTypes), []byte(newCastTypes))

			// Replace all instances of the api import path in gogoproto customtypes with the new import path
			currentCustomTypes := fmt.Sprintf(`(gogoproto.customtype) = "%v`, currentPath)
			newCustomTypes := fmt.Sprintf(`(gogoproto.customtype) = "%v`, newPath)
			updatedData = bytes.ReplaceAll(updatedData, []byte(currentCustomTypes), []byte(newCustomTypes))

			fileMode := d.Type().Perm()
			if err := os.WriteFile(path, updatedData, fileMode); err != nil {
				return trace.Wrap(err)
			}

			addRollBackFunc(func() error {
				err := os.WriteFile(path, data, fileMode)
				return trace.Wrap(err, "failed to rollback changes to proto file %v", path)
			})
		}
		return nil
	})
}

// getNewModImportPath gets the new import path given a go module import path and the updated version
func getNewModImportPath(oldPath string, newVersion *semver.Version) string {
	// get the new major version suffix - e.g "" for v0/v1 or "/vX" for vX where X >= 2
	var majVerSuffix string
	if newVersion.Major >= 2 {
		majVerSuffix = fmt.Sprintf("/v%d", newVersion.Major)
	}

	// get the new mod path by replacing the current mod path with the new major version suffix
	newPath := oldPath + majVerSuffix
	if suffixIndex := strings.Index(oldPath, "/v"); suffixIndex != -1 {
		newPath = oldPath[:suffixIndex] + majVerSuffix
	}
	return newPath
}

// rollBackFuncs are used to revert changes if the program fails with an error.
type rollBackFunc func() error
type addRollBackFunc func(rollBackFunc)

// log error, rollback any changes made, and exit with non-zero status code
func exitWithError(err error, rollBackFuncs []rollBackFunc) {
	if err != nil {
		log.WithError(err).Error()
	}
	if rollBackFuncs != nil {
		log.Info("Rolling back changes...")
		for _, r := range rollBackFuncs {
			if err := r(); err != nil {
				log.WithError(err).Error()
			}
		}
	}
	os.Exit(1)
}

// log message and exit with non-zero status code
func exitWithMessage(format string, args ...interface{}) {
	log.Infof(format, args...)
	os.Exit(1)
}
