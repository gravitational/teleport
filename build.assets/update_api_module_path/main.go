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
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport/api/v7"

	"github.com/gravitational/trace"
	"golang.org/x/mod/modfile"
)

const (
	apiModFilePath = "./api/go.mod"
	makefilePath   = "./Makefile"
)

func main() {
	// the api mod path should only be updated on releases, check for non-release suffixes
	if strings.Contains(api.Version, "-") {
		exitWithMessage("the current api version is not a release, continue without updating")
	}

	// get old api mod path from `go.mod`
	oldModPath, err := getModPath(apiModFilePath)
	if err != nil {
		exitWithError(trace.Wrap(err, "could not get mod path"))
	}

	// get new api mod path using version in `version.go`
	newModPath, isNew := getNewPath(oldModPath)
	if !isNew {
		exitWithMessage("the api module path has not changed, continue without updating")
	}

	// replace old mod path with new mod path in .go, .proto, and go.mod files.
	if err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if strings.HasSuffix(path, ".go") {
			err := replaceInFile(path, oldModPath, newModPath)
			return trace.Wrap(err)
		}
		if strings.HasSuffix(path, ".proto") {
			err := replaceInFile(path, oldModPath, newModPath)
			return trace.Wrap(err)
		}
		if strings.HasSuffix(path, "go.mod") {
			err := replaceInModFile(path, oldModPath, newModPath)
			return trace.Wrap(err)
		}
		return nil
	}); err != nil {
		exitWithError(trace.Wrap(err, "failed to update files"))
	}

	// update the api vendor symlink line in `make update-vendor`
	oldLinkLine := "ln -s -r $(shell readlink -f api) vendor/" + oldModPath
	newLinkLine := "ln -s -r $(shell readlink -f api) vendor/" + newModPath
	if err := replaceInFile(makefilePath, oldLinkLine, newLinkLine); err != nil {
		exitWithError(trace.Wrap(err, "failed to update Makefile"))
	}

	exitWithMessage("successfully updated api version")
}

// replaceInFile updates instances of oldModPath to newModPath in a .go file.
func replaceInFile(path, oldModPath, newModPath string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	fileString := strings.ReplaceAll(string(data), oldModPath, newModPath)
	err = ioutil.WriteFile(path, []byte(fileString), 0660)
	return trace.Wrap(err)
}

// replaceInModFile updates instances of oldModPath to newModPath in a go.mod file.
// The modFile is updated in place by updating the syntax fields directly.
func replaceInModFile(path, oldModPath, newModPath string) error {
	modFile, err := getModFile(path)
	if err != nil {
		return trace.Wrap(err)
	}
	// Update mod name if needed
	if modFile.Module.Syntax.Token[1] == oldModPath {
		modFile.Module.Syntax.Token[1] = newModPath
	}
	// Update require statements if needed
	for _, r := range modFile.Require {
		if r.Mod.Path == oldModPath {
			pathI, versionI := 0, 1
			if r.Syntax.Token[0] == "require" {
				pathI, versionI = 1, 2
			}
			r.Syntax.Token[pathI], r.Syntax.Token[versionI] = newModPath, currentVersionString()
		}
	}
	// Update replace statements if needed
	for _, r := range modFile.Replace {
		if r.Old.Path == oldModPath {
			pathI := 0
			if r.Syntax.Token[0] == "replace" {
				pathI = 1
			}
			r.Syntax.Token[pathI] = newModPath
		}
	}
	// Format and save mod file
	bts, err := modFile.Format()
	if err != nil {
		return trace.Wrap(err, "could not format go.mod file with new import path")
	}
	if err = ioutil.WriteFile(path, bts, 0660); err != nil {
		return trace.Wrap(err, "could not rewrite go.mod file")
	}
	return nil
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

// getModFile returns an AST of the given go.mod file
func getModFile(path string) (*modfile.File, error) {
	bts, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f, err := modfile.Parse(path, bts, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return f, nil
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

// currentVersionSuffix returns the current api version in the format "/vX", or empty if < v2
func currentVersionSuffix() string {
	versionSuffix := "/" + strings.Split(currentVersionString(), ".")[0]
	switch versionSuffix {
	case "/v0", "/v1":
		return ""
	}
	return versionSuffix
}

func exitWithError(err error) {
	if err != nil {
		log.New(os.Stderr, "", 0).Print(err)
	}
	os.Exit(1)
}

func exitWithMessage(message string) {
	if message != "" {
		log.New(os.Stdout, "", 0).Print(message)
	}
	os.Exit(0)
}
