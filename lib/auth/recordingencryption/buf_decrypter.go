// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package recordingencryption

import (
	"bytes"
	"context"
	"io"

	"github.com/gravitational/trace"
)

// ageEncryptionPrefix is the prefix used to identify age-encrypted data.
// age always uses "age-encryption.org/v1" as the prefix for its encrypted files.
const ageEncryptionPrefix = "age-encryption.org"

var ageEncryptionPrefixBytes = []byte(ageEncryptionPrefix)

// Decrypter wraps an io.Reader with decryption if the data is age-encrypted.
type Decrypter interface {
	WithDecryption(ctx context.Context, reader io.Reader) (io.Reader, error)
}

// DecryptBufferIfEncrypted checks whether the provided buffer contains
// age-encrypted data and decrypts it if necessary.
//
// The function looks for the standard age encryption header prefix to determine
// whether the data is encrypted. If the buffer is not age-encrypted, it returns
// the original data unchanged.
//
// If the buffer is encrypted, a Decrypter must be provided. The function uses
// the Decrypter to read and decrypt the buffer, returning the plaintext bytes.
// If no Decrypter is configured when encrypted data is detected, an error is
// returned.
func DecryptBufferIfEncrypted(ctx context.Context, buf []byte, decrypter Decrypter) ([]byte, error) {
	if !bytes.HasPrefix(buf, ageEncryptionPrefixBytes) {
		return buf, nil
	}

	if decrypter == nil {
		return nil, trace.BadParameter("recording metadata decrypter is not configured")
	}

	decryptedReader, err := decrypter.WithDecryption(ctx, bytes.NewReader(buf))
	if err != nil {
		return nil, trace.Wrap(err, "decrypting recording metadata")
	}

	decryptedBuf := bytes.NewBuffer(make([]byte, 0, len(buf)))
	_, err = io.Copy(decryptedBuf, decryptedReader)
	if err != nil {
		return nil, trace.Wrap(err, "reading decrypted recording metadata")
	}

	return decryptedBuf.Bytes(), nil
}
