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

package main

import (
	"net/http"
	"os"
)

// githubAuthTransport implements GraphQL HTTP transport with HTTP auth
type githubAuthTransport struct {
	wrapped http.RoundTripper
}

// RoundTrip sets Authorization header from GITHUB_TOKEN env variable
func (t *githubAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := os.Getenv("GITHUB_TOKEN")
	req.Header.Set("Authorization", "token "+key)
	return t.wrapped.RoundTrip(req)
}
