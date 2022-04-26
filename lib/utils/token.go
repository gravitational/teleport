/*
Copyright 2019 Gravitational, Inc.

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

package utils

import (
	"os"
	"strings"

	"github.com/gravitational/trace"
)

// ReadToken is a utility function to read the token
// from the disk if it looks like a path,
// otherwise, treat it as a value
func ReadToken(token string) (string, error) {
	if !strings.HasPrefix(token, "/") {
		return token, nil
	}
	// treat it as a file
	out, err := os.ReadFile(token)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	// trim newlines as tokens in files tend to have newlines
	return strings.TrimSpace(string(out)), nil
}
