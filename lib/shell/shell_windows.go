// +build windows

/*
Copyright 2018 Gravitational, Inc.

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

package shell

import (
	"github.com/gravitational/trace"
)

// getLoginShell always return an error on Windows. This code his behind a
// build flag to allow cross compilation (Unix version uses CGO).
func getLoginShell(username string) (string, error) {
	return "", trace.BadParameter("login shell on Windows is not supported")
}
