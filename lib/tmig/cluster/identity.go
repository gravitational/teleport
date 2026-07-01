// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import "fmt"

// PinIdentity validates that SOURCE and TARGET are different clusters.
// This is a hard gate: a mix-up could mint tokens or disable units on the wrong cluster.
func PinIdentity(source, target *PingResult) error {
	if source.ClusterID == target.ClusterID {
		return fmt.Errorf("SOURCE and TARGET resolve to the same cluster (%s, id=%s): "+
			"refusing to proceed — check proxy addresses and identities",
			source.ClusterName, source.ClusterID)
	}
	return nil
}
