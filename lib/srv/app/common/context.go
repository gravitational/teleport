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

package common

import (
	"context"
	"net/http"
)

// WithAWSAssumedRole adds AWS assumed role to the context of the provided request.
func WithAWSAssumedRole(r *http.Request, assumedRole string) *http.Request {
	if assumedRole == "" {
		return r
	}
	return r.WithContext(context.WithValue(
		r.Context(),
		contextKeyAWSAssumedRole,
		assumedRole,
	))
}

// GetAWSAssumedRole gets AWS assumed role from a request.
func GetAWSAssumedRole(r *http.Request) string {
	assumedRoleValue := r.Context().Value(contextKeyAWSAssumedRole)
	assumedRole, ok := assumedRoleValue.(string)
	if ok {
		return assumedRole
	}
	return ""
}

type contextKey string

// contextKeyAWSAssumedRole is the context key for AWS assumed role.
const contextKeyAWSAssumedRole contextKey = "aws-assumed-role"
