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

import (
	"encoding/json"
	"fmt"
)

// Info describes build version with a semver-complaint version string and
// git-related commit/tree state details.
type Info struct {
	Version      string `json:"version"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
}

// Get returns current build version.
func Get() Info {
	return Info{
		Version:      version,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
	}
}

func (r Info) String() string {
	return r.Version
}

// Print prints build version in default format.
func Print() {
	payload, err := json.Marshal(Get())
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", payload)
}
