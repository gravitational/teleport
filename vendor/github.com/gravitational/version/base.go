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
package version

// Version value defaults
var (
	// Version string, a slightly modified version of `git describe` to be semver-complaint
	version      string = "v0.0.0-master+$Format:%h$"
	gitCommit    string = "$Format:%H$"    // sha1 from git, output of $(git rev-parse HEAD)
	gitTreeState string = "not a git tree" // state of git tree, either "clean" or "dirty"
)

// Init sets an alternative default for the version string.
func Init(baseVersion string) {
	version = baseVersion
}
