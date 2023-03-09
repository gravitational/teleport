// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lib

import (
	"fmt"
	"runtime"
)

// PrintVersion prints the specified app version to STDOUT
func PrintVersion(appName string, version string, gitref string) {
	if gitref != "" {
		fmt.Printf("%v v%v git:%v %v\n", appName, version, gitref, runtime.Version())
	} else {
		fmt.Printf("%v v%v %v\n", appName, version, runtime.Version())
	}
}
