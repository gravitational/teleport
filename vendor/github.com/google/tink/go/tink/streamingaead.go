// Copyright 2020 Google LLC
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

import "io"

/*
StreamingAEAD is an interface for streaming authenticated encryption with associated data.

Streaming encryption is typically used for encrypting large plaintexts such as large files.
Tink may eventually contain multiple interfaces for streaming encryption depending on the
supported properties. This interface supports a streaming interface for symmetric encryption with
authentication. The underlying encryption modes are selected so that partial plaintext can be
obtained fast by decrypting and authenticating just a part of the ciphertext.

Instances of StreamingAead must follow the OAE2 definition as proposed in the paper "Online
Authenticated-Encryption and its Nonce-Reuse Misuse-Resistance" by Hoang, Reyhanitabar, Rogaway
and Vizár https://eprint.iacr.org/2015/189.pdf
*/
type StreamingAEAD interface {
	// NewEncryptingWriter returns a wrapper around underlying io.Writer, such that any write-operation
	// via the wrapper results in AEAD-encryption of the written data, using aad
	// as associated authenticated data. The associated data is not included in the ciphertext
	// and has to be passed in as parameter for decryption.
	NewEncryptingWriter(w io.Writer, aad []byte) (io.WriteCloser, error)

	// NewDecryptingReader returns a wrapper around underlying io.Reader, such that any read-operation
	// via the wrapper results in AEAD-decryption of the underlying ciphertext,
	// using aad as associated authenticated data.
	NewDecryptingReader(r io.Reader, aad []byte) (io.Reader, error)
}
