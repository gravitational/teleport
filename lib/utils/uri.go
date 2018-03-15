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

package utils

import (
	"net/url"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

// ParseSessionsURI parses uri per convention of session upload URIs
// file is a default scheme
func ParseSessionsURI(in string) (*url.URL, error) {
	if in == "" {
		return nil, trace.BadParameter("uri is empty")
	}
	u, err := url.Parse(in)
	if err != nil {
		return nil, trace.BadParameter("failed to parse URI %q: %v", in, err)
	}
	if u.Scheme == "" {
		u.Scheme = teleport.SchemeFile
	}
	return u, nil
}
