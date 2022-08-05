/*
Copyright 2021 Gravitational, Inc.

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

package env

// Event is a GitHub event. See the following more more details:
// https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads
type Event struct {
	Action string `json:"action"`

	Repository  Repository  `json:"repository"`
	PullRequest PullRequest `json:"pull_request"`
}

type Repository struct {
	Name  string `json:"name"`
	Owner Owner  `json:"owner"`
}

type Owner struct {
	Login string `json:"login"`
}

type PullRequest struct {
	User   User `json:"user"`
	Number int  `json:"number"`

	// UnsafeHead can be attacker controlled and should not be used in any
	// security sensitive context. See the following link for more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeHead Head `json:"head"`
}

type User struct {
	Login string `json:"login"`
}

type Head struct {
	// UnsafeSHA can be attacker controlled and should not be used in any
	// security sensitive context. See the following link for more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeSHA string `json:"sha"`

	// UnsafeRef can be attacker controlled and should not be used in any
	// security sensitive context. See the following link for more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeRef string `json:"ref"`
}
