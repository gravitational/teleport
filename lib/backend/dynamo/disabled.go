// +build !dynamodb

/*
This file contains "plugs" i.e. empty entry points for DynamoDB
back-end. The only thing they do is to return "built without DynamoDB support" error

Copyright 2015 Gravitational, Inc.

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

package dynamo

import (
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
)

const (
	missingBuildTag = "DynamoDB backend was not enabled during Teleport build"
)

func FromJSON(paramsJSON string) (backend.Backend, error) {
	return nil, trace.NotFound(missingBuildTag)
}

func ConfigureBackend(*backend.Config) (string, error) {
	return "", trace.NotFound(missingBuildTag)
}
