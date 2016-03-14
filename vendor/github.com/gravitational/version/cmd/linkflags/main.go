/*
Copyright 2015 Gravitational, Inc.

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
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/version"
	"github.com/gravitational/version/pkg/tool"
)

// pkg is the path to the package the tool will create linker flags for.
var pkg = flag.String("pkg", "", "root package path")

// versionPackage is the path to this version package.
// It is used to access version information attributes during link time.
// This flag is useful when the version package is custom-vendored and has a different package path.
var versionPackage = flag.String("verpkg", "github.com/gravitational/version", "path to the version package")

var compatMode = flag.Bool("compat", false, "generate linker flags using go1.4 syntax")

// semverPattern defines a regexp pattern to modify the results of `git describe` to be semver-complaint.
var semverPattern = regexp.MustCompile(`(.+)-([0-9]{1,})-g([0-9a-f]{14})$`)

// goVersionPattern defines a regexp pattern to parse versions of the `go tool`.
var goVersionPattern = regexp.MustCompile(`go([1-9])\.(\d+)(?:.\d+)*`)

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	log.SetFlags(0)
	flag.Parse()
	if *pkg == "" {
		return fmt.Errorf("-pkg required")
	}

	goVersion, err := goToolVersion()
	if err != nil {
		return fmt.Errorf("failed to determine go tool version: %v\n", err)
	}

	info, err := getVersionInfo(*pkg)
	if err != nil {
		return fmt.Errorf("failed to determine version information: %v\n", err)
	}

	var linkFlags []string
	linkFlag := func(key, value string) string {
		if goVersion <= 14 || *compatMode {
			return fmt.Sprintf("-X %s.%s %s", *versionPackage, key, value)
		} else {
			return fmt.Sprintf("-X %s.%s=%s", *versionPackage, key, value)
		}
	}

	// Determine the values of version-related variables as commands to the go linker.
	if info.GitCommit != "" {
		linkFlags = append(linkFlags, linkFlag("gitCommit", info.GitCommit))
		linkFlags = append(linkFlags, linkFlag("gitTreeState", info.GitTreeState))
	}
	if info.Version != "" {
		linkFlags = append(linkFlags, linkFlag("version", info.Version))
	}

	fmt.Printf("%s", strings.Join(linkFlags, " "))
	return nil
}

// getVersionInfo collects the build version information for package pkg.
func getVersionInfo(pkg string) (*version.Info, error) {
	git := newGit(pkg)
	commitID, err := git.commitID()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain git commit ID: %v\n", err)
	}
	treeState, err := git.treeState()
	if err != nil {
		return nil, fmt.Errorf("failed to determine git tree state: %v\n", err)
	}
	tag, err := git.tag(commitID)
	if err != nil {
		tag = ""
	}
	if tag != "" {
		tag = semverify(tag)
		if treeState == dirty {
			tag = tag + "-" + string(treeState)
		}
	}
	return &version.Info{
		Version:      tag,
		GitCommit:    commitID,
		GitTreeState: string(treeState),
	}, nil
}

// goToolVersion determines the version of the `go tool`.
func goToolVersion() (toolVersion, error) {
	goTool := &tool.T{Cmd: "go"}
	out, err := goTool.Exec("version")
	if err != nil {
		return toolVersionUnknown, err
	}
	build := strings.Split(out, " ")
	if len(build) > 2 {
		return parseToolVersion(build[2]), nil
	}
	return toolVersionUnknown, nil
}

// parseToolVersion translates a string version of the form 'go1.4.3' to a numeric value 14.
func parseToolVersion(version string) toolVersion {
	match := goVersionPattern.FindStringSubmatch(version)
	if len(match) > 2 {
		// After a successful match, match[1] and match[2] are integers
		major := mustAtoi(match[1])
		minor := mustAtoi(match[2])
		return toolVersion(major*10 + minor)
	}
	return toolVersionUnknown
}

func newGit(pkg string) *git {
	args := []string{"--work-tree", pkg, "--git-dir", filepath.Join(pkg, ".git")}
	return &git{&tool.T{
		Cmd:  "git",
		Args: args,
	}}
}

// git represents an instance of the git tool.
type git struct {
	*tool.T
}

// treeState describes the state of the git tree.
// `git describe --dirty` only considers changes to existing files.
// We track tree state and consider untracked files as they also affect the build.
type treeState string

const (
	clean treeState = "clean"
	dirty           = "dirty"
)

// toolVersion represents a tool version as an integer.
// toolVersion only considers the first two significant version parts and is computed as follows:
// 	majorVersion*10+minorVersion
type toolVersion int

const toolVersionUnknown toolVersion = 0

func (r *git) commitID() (string, error) {
	return r.Exec("rev-parse", "HEAD^{commit}")
}

func (r *git) treeState() (treeState, error) {
	out, err := r.Exec("status", "--porcelain")
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return clean, nil
	}
	return dirty, nil
}

func (r *git) tag(commitID string) (string, error) {
	return r.Exec("describe", "--tags", "--abbrev=14", commitID+"^{commit}")
}

// semverify transforms the output of `git describe` to be semver-complaint.
func semverify(version string) string {
	var result []byte
	match := semverPattern.FindStringSubmatchIndex(version)
	if match != nil {
		return string(semverPattern.ExpandString(result, "$1.$2+$3", string(version), match))
	}
	return version
}

// mustAtoi converts value to an integer.
// It panics if the value does not represent a valid integer.
func mustAtoi(value string) int {
	result, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return result
}
