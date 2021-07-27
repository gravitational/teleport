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
	"go/format"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport/api/v7"

	"github.com/gravitational/trace"
	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

const (
	teleportDir          = "./"
	apiDir               = "./api"
	apiExampleDir        = "./examples/go-client"
	bpfDir               = "./lib/bpf"
	restrictedsessionDir = "./lib/restrictedsession"
	makefilePath         = "./Makefile"
)

func main() {
	// the api mod path should only be updated on releases
	if strings.Contains(api.Version, "-") {
		exitWithError(trace.Errorf("the current api version is not a release"))
	}

	// get current module name from `go.mod`
	oldModPath, err := getModPath(apiDir)
	if err != nil {
		exitWithError(trace.Wrap(err, "could not get mod path"))
	}

	// get new module name from using updated version in `version.go`
	newModPath, isNew := getNewPath(oldModPath)
	if !isNew {
		exitWithError(trace.Errorf("the api module path has not changed"))
	}

	// update go files
	if err := updatePackages(teleportDir, oldModPath, newModPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport packages"))
	}
	if err := updatePackages(apiDir, oldModPath, newModPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update api packages"))
	}
	if err := updatePackages(apiExampleDir, oldModPath, newModPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update api example packages"))
	}

	// manually update bpf and restrictedsession files since they aren't discoverable through go/packages
	if err := updateFile(bpfDir, oldModPath, newModPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update bpf package"))
	}
	if err := updateFile(restrictedsessionDir, oldModPath, newModPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update restrictedsession package"))
	}

	// update go.mod files
	if err := updateModFile(teleportDir, oldModPath, newModPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update teleport mod file"))
	}
	if err := updateModFile(apiDir, oldModPath, newModPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update api mod file"))
	}
	if err := updateModFile(apiExampleDir, oldModPath, newModPath); err != nil {
		exitWithError(trace.Wrap(err, "failed to update api example mod file"))
	}

	// update `make update-vendor` to use the new import path and then run it.
	data, err := ioutil.ReadFile(makefilePath)
	if err != nil {
		exitWithError(trace.Wrap(err, "failed to update Makefile"))
	}
	fileString := strings.ReplaceAll(string(data), oldModPath, newModPath)
	if err := ioutil.WriteFile(makefilePath, []byte(fileString), 0660); err != nil {
		exitWithError(trace.Wrap(err, "failed to update Makefile"))
	}
	if err := exec.Command("make", "update-vendor").Run(); err != nil {
		exitWithError(trace.Wrap(err, "failed to update vendor"))
	}
}

func exitWithError(err error) {
	if err != nil {
		log.New(os.Stderr, "", 1).Println(err)
	}
	os.Exit(1)
}

// getModPath gets the module's currently set path/name
func getModPath(dir string) (string, error) {
	modFile, err := getModFile(dir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if modFile.Module.Mod.Path == "" {
		return "", trace.Errorf("could not get mod path")
	}
	return modFile.Module.Mod.Path, nil
}

// getModFile returns an AST of the given directory's go.mod file
func getModFile(dir string) (*modfile.File, error) {
	bts, err := ioutil.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f, err := modfile.Parse(filepath.Join(dir, "go.mod"), bts, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return f, nil
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
			r.Syntax.Token[pathI], r.Syntax.Token[versionI] = newPath, currentVersionString()
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
		return errors.Wrap(err, "could not format go.mod file with new import path")
	}
	if err = ioutil.WriteFile(filepath.Join(dir, "go.mod"), bts, 0660); err != nil {
		return errors.Wrap(err, "could not rewrite go.mod file")
	}
	return nil
}

// updateImportPath updates instances of the oldModPath with the newModPath in the given packages.
func updatePackages(dir, oldModPath, newModPath string) error {
	mode := packages.NeedTypes | packages.NeedSyntax
	c := &packages.Config{Mode: mode, Tests: true, Dir: dir}
	pkgs, err := packages.Load(c, "./...")
	if err != nil {
		return trace.Wrap(err)
	}

	errChan := make(chan error, 0)
	go func() {
		packages.Visit(pkgs, func(pkg *packages.Package) bool {
			if err = updateImportPath(pkg, oldModPath, newModPath); err != nil {
				select {
				case errChan <- trace.Wrap(err):
				default:
				}
				return false
			}
			return true
		}, nil)
		close(errChan)
	}()

	return trace.Wrap(<-errChan)
}

// updateImportPath updates instances of the oldModPath with the newModPath in the given package.
func updateImportPath(p *packages.Package, oldModPath, newModPath string) error {
	for _, syn := range p.Syntax {
		var rewritten bool
		for _, i := range syn.Imports {
			imp := strings.Replace(i.Path.Value, "\"", "", 2)
			if strings.HasPrefix(imp, oldModPath) && !strings.HasPrefix(imp, newModPath) {
				newImp := strings.Replace(imp, oldModPath, newModPath, 1)
				if astutil.RewriteImport(p.Fset, syn, imp, newImp) {
					rewritten = true
				}
			}
		}
		if !rewritten {
			continue
		}

		goFileName := p.Fset.File(syn.Pos()).Name()
		f, err := os.Create(goFileName)
		if err != nil {
			return errors.Wrapf(err, "could not create go file %v", goFileName)
		}
		defer f.Close()

		if err = format.Node(f, p.Fset, syn); err != nil {
			return errors.Wrapf(err, "could not rewrite go file %v", goFileName)
		}
	}

	return nil
}

// updateFile updates instances of the oldModPath with the newModPath in go files in the given directory.
func updateFile(dir, oldModPath, newModPath string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(d.Name(), ".go") {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			fileString := strings.ReplaceAll(string(data), oldModPath, newModPath)
			if err := ioutil.WriteFile(path, []byte(fileString), 0660); err != nil {
				return err
			}
		}
		return nil
	})
}

// getNewPath updates the module path with the current version,
// and returns whether or not it changed.
func getNewPath(path string) (string, bool) {
	newPath := trimVersionSuffix(path) + currentVersionSuffix()
	if newPath == path {
		return "", false
	}
	return newPath, true
}

// trimVersionSuffix trims the version suffix "/vX" off of the end of an import path.
func trimVersionSuffix(path string) string {
	splitPath := strings.Split(path, "/")
	last := splitPath[len(splitPath)-1]
	if strings.HasPrefix(last, "v") {
		return strings.Join(splitPath[:len(splitPath)-1], "/")
	}
	return path
}

// currentVersionString returns the current api version in the format "vX.Y.Z"
func currentVersionString() string {
	return "v" + api.Version
}

// currentVersionSuffix returns the current api version in the format "/vX"
func currentVersionSuffix() string {
	return "/" + strings.Split(currentVersionString(), ".")[0]
}
