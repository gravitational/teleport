//go:build !piv && !pivtest

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
	"errors"

	"github.com/gravitational/trace"
)

var errPIVUnavailable = errors.New("PIV is unavailable in current build")

func getOrGenerateYubiKeyPrivateKey(ctx context.Context, policy PrivateKeyPolicy, slot PIVSlot) (*PrivateKey, error) {
	return nil, trace.Wrap(errPIVUnavailable)
}

func parseYubiKeyPrivateKeyData(keyDataBytes []byte) (*PrivateKey, error) {
	return nil, trace.Wrap(errPIVUnavailable)
}

func (s PIVSlot) validate() error {
	return trace.Wrap(errPIVUnavailable)
}

// IsHardware returns true if [k] is a hardware PIV key.
func (k *PrivateKey) IsHardware() bool {
	// Built without PIV support - this must be false.
	return false
}
