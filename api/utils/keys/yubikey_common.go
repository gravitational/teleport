/*
Copyright 2022 Gravitational, Inc.
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

package keys

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// GetYubiKeyPrivateKey attempt to retrieve a YubiKey private key matching the given hardware key policy
// from the given slot. If slot is unspecified, the default slot for the given key policy will be used.
// If the slot is empty, a new private key matching the given policy will be generated in the slot.
//   - hardware_key: 9a
//   - hardware_key_touch: 9c
//   - hardware_key_pin: 9d
//   - hardware_key_touch_pin: 9e
func GetYubiKeyPrivateKey(ctx context.Context, policy PrivateKeyPolicy, slot hardwarekey.PIVSlotKeyString, customPrompt hardwarekey.Prompt) (*PrivateKey, error) {
	priv, err := getOrGenerateYubiKeyPrivateKey(ctx, policy, slot, customPrompt)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get a YubiKey private key")
	}
	return priv, nil
}
