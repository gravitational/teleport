/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package testdata

// InvalidKeysBytes is a map of invalid keys to their byte representation.
var InvalidKeysBytes = map[string][]byte{
	"short-file": []byte("short file"),

	"empty-file": []byte(""),

	"invalid-key": []byte(`-----BEGIN PRIVATE
KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDQ7z7z7z7z7z7z
-----END OPENSSH PRIVATE KEY-----
`),

	"invalid-key-valid-headers": []byte(
		`-----BEGIN OPENSSH PRIVATE KEY-----
trash
-----END OPENSSH PRIVATE KEY-----
`),

	"invalid-key-invalid-header": []byte(
		`abcefg-----BEGIN OPENSSH PRIVATE KEY-----
-----END OPENSSH PRIVATE KEY-----
`),

	"valid-key-not-supported-header": []byte(`-----BEGIN RANDOM PRIVATE KEY-----
MHcCAQEEINGWx0zo6fhJ/0EAfrPzVFyFC9s18lBt3cRoEDhS3ARooAoGCCqGSM49
AwEHoUQDQgAEi9Hdw6KvZcWxfg2IDhA7UkpDtzzt6ZqJXSsFdLd+Kx4S3Sx4cVO+
6/ZOXRnPmNAlLUqjShUsUBBngG0u2fqEqA==
-----END EC PRIVATE KEY-----
`),
}
