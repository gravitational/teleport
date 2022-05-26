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
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// TryReadValueAsFile is a utility function to read a value
// from the disk if it looks like an absolute path,
// otherwise, treat it as a value.
// It only support absolute paths to avoid ambiguity in interpretation of the value
func TryReadValueAsFile(value string) (string, error) {
	if !filepath.IsAbs(value) {
		return value, nil
	}
	// treat it as an absolute filepath
	contents, err := os.ReadFile(value)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	// trim newlines as tokens in files tend to have newlines
	out := strings.TrimSpace(string(contents))

	if out == "" {
		log.Warnf("Empty config value file: %v", value)
	}
	return out, nil
}
