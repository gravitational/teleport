//go:build pivtest

// Copyright 2025 Gravitational, Inc.
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

package piv

import (
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// TODO(Joerger): Rather than using a global service, clients should be updated to
// create a single YubiKeyService and ensure it is reused across the program
// execution. At this point, it may make more sense to directly inject the mocked
// hardware key service into the test instead of using the pivtest build tag to do it.
var mockedHardwareKeyService = hardwarekey.NewMockHardwareKeyService(nil /*prompt*/)

// Returns a globally shared [hardwarekey.MockHardwareKeyService]. Test callers should
// prefer [hardwarekey.NewMockHardwareKeyService] when possible.
func NewYubiKeyService(prompt hardwarekey.Prompt) *hardwarekey.MockHardwareKeyService {
	mockedHardwareKeyService.SetPrompt(prompt)
	return mockedHardwareKeyService
}
