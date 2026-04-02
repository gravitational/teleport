// Copyright 2025 Open Container Initiative
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Original source: https://github.com/opencontainers/selinux/blob/main/go-selinux/selinux_linux.go#L229
//
// Modified to return an error.

package selinux

import (
	"bufio"
	"bytes"
	"os"

	"github.com/gravitational/trace"
)

func readConfig(target string) (string, error) {
	in, err := os.Open(selinuxConfig)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer in.Close()

	scanner := bufio.NewScanner(in)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			// Skip blank lines
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			// Skip comments
			continue
		}
		fields := bytes.SplitN(line, []byte{'='}, 2)
		if len(fields) != 2 {
			continue
		}
		if string(fields[0]) == target {
			return string(bytes.Trim(fields[1], `"`)), nil
		}
	}

	return "", nil
}
