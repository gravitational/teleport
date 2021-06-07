// Copyright 2018 Google LLC
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
//
////////////////////////////////////////////////////////////////////////////////

package tink

// Verifier is the verifying interface for digital signature.
//
// Implementations of this interface are secure against adaptive chosen-message
// attacks.  Signing data ensures authenticity and integrity of that data, but
// not its secrecy.
type Verifier interface {
	// Verifies returns nil if signature is a valid signature for data; otherwise returns an error.
	Verify(signature, data []byte) error
}
