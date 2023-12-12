/*
Copyright 2023 Gravitational, Inc.

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
	"context"
	"strings"
)

// StaticGetter is a fake version.Getter that return a static answer. This is used
// for testing purposes.
type StaticGetter struct {
	version string
	err     error
}

// GetVersion returns the statically defined version.
func (v StaticGetter) GetVersion(_ context.Context) (string, error) {
	return v.version, v.err
}

// NewStaticGetter creates a StaticGetter
func NewStaticGetter(version string, err error) Getter {
	semVersion := version
	if semVersion != "" && !strings.HasPrefix(semVersion, "v") {
		semVersion = "v" + version
	}
	return StaticGetter{
		version: semVersion,
		err:     err,
	}
}
