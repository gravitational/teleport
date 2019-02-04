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

package roundtrip

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// errorMessage is the error message to return when invalid input is provided by the caller.
const errorMessage = "invalid path, path can only be composed of characters, hyphens, slashes, and dots"

// whitelistPattern is the pattern of allowed characters for the path.
var whitelistPattern = regexp.MustCompile(`^[0-9A-Za-z@/_:.-]*$`)

// isPathSafe checks if the passed in path conforms to a whitelist.
func isPathSafe(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}

	e, err := url.PathUnescape(u.Path)
	if err != nil {
		return err
	}

	if strings.Contains(e, "..") {
		return fmt.Errorf(errorMessage)
	}

	if !whitelistPattern.MatchString(e) {
		return fmt.Errorf(errorMessage)
	}

	return nil
}
