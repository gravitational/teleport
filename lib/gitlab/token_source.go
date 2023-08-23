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

package gitlab

import "github.com/gravitational/trace"

type envGetter func(key string) string

// IDTokenSource allows a GitLab ID token to be fetched whilst executing
// within the context of a GitLab actions workflow.
type IDTokenSource struct {
	getEnv envGetter
}

func (its *IDTokenSource) GetIDToken() (string, error) {
	tok := its.getEnv("TBOT_GITLAB_JWT")
	if tok == "" {
		return "", trace.BadParameter(
			"TBOT_GITLAB_JWT environment variable missing",
		)
	}

	return tok, nil
}

func NewIDTokenSource(getEnv envGetter) *IDTokenSource {
	return &IDTokenSource{
		getEnv,
	}
}
