/*
Copyright 2022 Gravitational, Inc.

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
	"bufio"
	"io"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

func OSRelease(rel io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(rel)
	result := make(map[string]string)
	for scanner.Scan() {
		kvLine := scanner.Text()
		if kvLine == "" || kvLine[0] == '#' {
			continue
		}
		key, value, found := strings.Cut(kvLine, "=")
		if !found {
			continue
		}
		// unquote value if it is quoted (rhel, amzl)
		if value[0] == '"' {
			var err error
			value, err = strconv.Unquote(value)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		result[key] = value
	}
	return result, nil
}
