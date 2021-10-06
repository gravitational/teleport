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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func init() {
	utils.InitLogger(utils.LoggingForCLI, log.DebugLevel)
}

func main() {
	// the api module import path should only be updated on releases
	if isPreRelease() {
		exitWithMessage("the current API version (%v) is not a release, continue without updating", api.Version)
	}

	// get the current and new api module import paths
	currentPath, newPath, err := getAPIModuleImportPaths()
	if err != nil {
		exitWithError(trace.Wrap(err, "failed to get mod paths"))
	} else if currentPath == newPath {
		exitWithMessage("the api module path has not changed, continue without updating")
	}

	// update go files within the teleport/api and teleport modules to use the new import path
	log.Info("Updating teleport/api module...")
	if err := updateGoModule("./api", currentPath, newPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport/api module"))
	}
	log.Info("Updating teleport module...")
	if err := updateGoModule("./", currentPath, newPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport module"))
	}

	// Update .proto files in teleport/api to use the new import path
	log.Info("Updating .proto files...")
	if err := updateFiles("./api", ".proto", currentPath, newPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport mod file"))
	}
}

// updateGoModule updates instances of the currentPath with the newPath in the given go module.
func updateGoModule(modulePath, currentPath, newPath string) error {
	var buildFlags []string
	if len(os.Args) > 1 {
		buildFlags = os.Args[1:]
		log.Infof("    Using buildFlags: %v", buildFlags)
	} else {
		log.Info("    Updating without build flags. This will exclude some packages, such as /teleport/lib/bpf.")
	}

	mode := packages.NeedTypes | packages.NeedSyntax
	cfg := &packages.Config{Mode: mode, Tests: true, Dir: modulePath, BuildFlags: buildFlags}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return trace.Wrap(err)
	}

	log.Info("    Updating go files...")
	var errs []error
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if err = updateGoImports(pkg, currentPath, newPath); err != nil {
			errs = append(errs, err)
			return false
		}
		return true
	}, nil)

	if len(errs) != 0 {
		return trace.NewAggregate(errs...)
	}

	log.Info("    Updating go.mod...")
	if err := updateModFile(modulePath, currentPath, newPath); err != nil {
		return trace.Wrap(err, "failed to update mod file for module")
	}

	return nil
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
			return errors.Wrapf(err, "could not open go file %v", goFileName)
		}
		defer f.Close()

		if err = format.Node(f, p.Fset, syn); err != nil {
			return errors.Wrapf(err, "could not rewrite go file %v", goFileName)
		}
	}

	return nil
}

// updateModFile updates instances of oldPath to newPath in a go.mod file.
// The modFile is updated in place by updating the syntax fields directly.
func updateModFile(dir, oldPath, newPath string) error {
	modFile, err := getModFile(dir)
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
			pathI, versionI := 0, 1
			if r.Syntax.Token[0] == "require" {
				pathI, versionI = 1, 2
			}
			r.Syntax.Token[pathI], r.Syntax.Token[versionI] = newPath, "v"+api.Version
		}
	}
	// Update replace statements in place if needed
	for _, r := range modFile.Replace {
		if r.Old.Path == oldPath {
			pathI := 0
			if r.Syntax.Token[0] == "replace" {
				pathI = 1
			}
			r.Syntax.Token[pathI] = newPath
		}
	}
	// Format and save mod file
	bts, err := modFile.Format()
	if err != nil {
		return trace.Wrap(err, "could not format go.mod file with new import path")
	}
	if err = ioutil.WriteFile(filepath.Join(dir, "go.mod"), bts, 0660); err != nil {
		return trace.Wrap(err, "could not rewrite go.mod file")
	}
	return nil
}

// updateFiles updates instances of the currentPath with the newPath
// in files with the given fileExtension in the given directory.
func updateFiles(dir, fileExtension, currentPath, newPath string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if strings.HasSuffix(d.Name(), fileExtension) {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return trace.Wrap(err)
			}
			data = bytes.ReplaceAll(data, []byte(currentPath), []byte(newPath))
			if err := ioutil.WriteFile(path, data, 0660); err != nil {
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
	bts, err := ioutil.ReadFile(modPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f, err := modfile.Parse(modPath, bts, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return f, nil
}

// getAPIModuleImportPaths gets the current and new import paths for the api module
func getAPIModuleImportPaths() (current string, new string, err error) {
	// get the current mod path from `api/go.mod`
	currentPath, err := getModImportPath("./api")
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	// get the new major version suffix - e.g "" for v0/v1 or "/vX" for vX where X >= 2
	var majVerSuffix string
	if ver := semver.New(api.Version); ver.Major >= 2 {
		majVerSuffix = fmt.Sprintf("/v%d", ver.Major)
	}

	// get the new mod path by replacing the current mod path with the new major version suffix
	newPath := currentPath + majVerSuffix
	if suffixIndex := strings.Index(currentPath, "/v"); suffixIndex != -1 {
		newPath = currentPath[:suffixIndex] + majVerSuffix
	}

	return currentPath, newPath, nil
}

// returns whether the current api version is a pre-release, e.g "v7.0.0-beta"
func isPreRelease() bool {
	return semver.New(api.Version).PreRelease != ""
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
