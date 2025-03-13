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

package hardwarekey

import (
	"bytes"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"

	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
)

// AttestationStatement is an attestation statement for a hardware private key
// that supports json marshaling through the standard json/encoding package.
type AttestationStatement attestationv1.AttestationStatement

// ToProto converts this AttestationStatement to its protobuf form.
func (ar *AttestationStatement) ToProto() *attestationv1.AttestationStatement {
	return (*attestationv1.AttestationStatement)(ar)
}

// AttestationStatementFromProto converts an AttestationStatement from its protobuf form.
func AttestationStatementFromProto(att *attestationv1.AttestationStatement) *AttestationStatement {
	return (*AttestationStatement)(att)
}

// MarshalJSON implements custom protobuf json marshaling.
func (ar *AttestationStatement) MarshalJSON() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := (&jsonpb.Marshaler{}).Marshal(buf, ar.ToProto())
	return buf.Bytes(), trace.Wrap(err)
}

// UnmarshalJSON implements custom protobuf json unmarshaling.
func (ar *AttestationStatement) UnmarshalJSON(buf []byte) error {
	return jsonpb.Unmarshal(bytes.NewReader(buf), ar.ToProto())
}
