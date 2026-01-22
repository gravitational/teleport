// Copyright 2026 Gravitational, Inc.
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

package types

import (
	"testing"
)

// TODO(codingllama): DELETE IN 20. This won't matter once the migration doesn't exist.
func TestCertAuthTypes_WindowsBeforeUser(t *testing.T) {
	var foundUser bool
	for _, caType := range CertAuthTypes {
		switch caType {
		case UserCA:
			foundUser = true
		case WindowsCA:
			if !foundUser {
				return // OK, Windows before User.
			}
			t.Errorf("" +
				"CertAuthTypes: UserCA appears before WindowsCA. " +
				"It's important for WindowsCA to be created before UserCA in new clusters, so it's not incorrectly cloned from UserCA if initial CA creation is only partly successful.")
		}
	}
}
