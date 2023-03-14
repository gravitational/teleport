// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lib

import (
	"github.com/gravitational/trace"
	"github.com/hashicorp/go-version"

	"github.com/gravitational/teleport/api/client/proto"
)

// AssertServerVersion returns an error if server version in ping response is
// less than minimum required version.
func AssertServerVersion(pong proto.PingResponse, minVersion string) error {
	actual, err := version.NewVersion(pong.ServerVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	required, err := version.NewVersion(minVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	if actual.LessThan(required) {
		return trace.Errorf("server version %s is less than %s", pong.ServerVersion, minVersion)
	}
	return nil
}
