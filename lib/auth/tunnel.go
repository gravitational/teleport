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

package auth

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// ValidateReverseTunnel validates the OIDC connector and sets default values
func ValidateReverseTunnel(rt types.ReverseTunnel) error {
	if err := rt.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	for _, addr := range rt.GetDialAddrs() {
		if _, err := utils.ParseAddr(addr); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
