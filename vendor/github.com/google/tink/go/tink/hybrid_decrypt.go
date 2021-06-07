// Copyright 2017 Google Inc.
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

/*
HybridDecrypt Interface for hybrid decryption.

Hybrid Encryption combines the efficiency of symmetric encryption with the convenience of
public-key encryption: to encrypt a message a fresh symmetric key is generated and used to
encrypt the actual plaintext data, while the recipient’s public key is used to encrypt the
symmetric key only, and the final ciphertext consists of the symmetric ciphertext and the
encrypted symmetric key.

WARNING

Hybrid Encryption does not provide authenticity, that is the recipient of an encrypted message
does not know the identity of the sender. Similar to general public-key encryption schemes the
security goal of Hybrid Encryption is to provide privacy only. In other words, Hybrid Encryption
is secure if and only if the recipient can accept anonymous messages or can rely on other
mechanisms to authenticate the sender.

Security guarantees

The functionality of Hybrid Encryption is represented as a pair of primitives (interfaces):
HybridEncrypt for encryption of data, and HybridDecrypt for decryption.
Implementations of these interfaces are secure against adaptive chosen ciphertext attacks. In
addition to plaintext the encryption takes an extra parameter contextInfo, which
usually is public data implicit from the context, but should be bound to the resulting
ciphertext, i.e. the ciphertext allows for checking the integrity of contextInfo (but
there are no guarantees wrt. the secrecy or authenticity of contextInfo).

contextInfo can be empty or null, but to ensure the correct decryption of a ciphertext
the same value must be provided for the decryption operation as was used during encryption
(HybridEncrypt).

A concrete instantiation of this interface can implement the binding of contextInfo to
the ciphertext in various ways, for example:


  use contextInfo as "associated data"-input for the employed AEAD symmetric
      encryption (cf. https://tools.ietf.org/html/rfc5116).
  use contextInfo as "CtxInfo"-input for HKDF (if the implementation uses HKDF as key
      derivation function, cf. https://tools.ietf.org/html/rfc5869).

*/
type HybridDecrypt interface {
	/**
	 * Decrypt operation: decrypts ciphertext verifying the integrity of contextInfo.
	 * returns resulting plaintext
	 */
	Decrypt(ciphertext, contextInfo []byte) ([]byte, error)
}
