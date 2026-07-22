/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package cryptoutils

import (
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// EncryptedPayload is the plaintext structure inside the KMS envelope.
// It binds the secret to the owning user and resource to prevent
// cross-user or cross-resource token theft.
type EncryptedPayload struct {
	User     string           `json:"user"`
	Resource types.ResourceID `json:"resource"`
	Payload  []byte           `json:"payload"`
}

// MarshalEncryptedPayload serializes an EncryptedPayload to JSON.
func MarshalEncryptedPayload(p *EncryptedPayload) ([]byte, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalEncryptedPayload deserializes an EncryptedPayload from JSON.
func UnmarshalEncryptedPayload(data []byte) (*EncryptedPayload, error) {
	var p EncryptedPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, trace.Wrap(err)
	}
	return &p, nil
}
