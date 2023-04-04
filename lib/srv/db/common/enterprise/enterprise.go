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

package enterprise

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
)

// ProtocolValidation checks if protocol is supported for current build.
func ProtocolValidation(dbProtocol string) error {
	switch dbProtocol {
	case defaults.ProtocolOracle:
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return trace.BadParameter("%s database protocol is only available with an enterprise license", dbProtocol)
		}
	}
	return nil
}
