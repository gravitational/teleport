/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

#ifndef CREDENTIAL_INFO_H_
#define CREDENTIAL_INFO_H_

// CredentialInfo represents a credential stored in the Secure Enclave.
typedef struct CredentialInfo {
  // label is the label for the Keychain entry.
  // In practice, the label is a combination of RPID and username.
  const char *label;

  // app_label is the application label for the Keychain entry.
  // In practice, the app_label is the credential ID.
  const char *app_label;

  // app_tag is the application tag for the Keychain entry.
  // In practice, the app_tag is the WebAuthn user handle.
  const char *app_tag;

  // pub_key_b64 is the public key representation, encoded as a standard base64
  // string.
  // Refer to
  // https://developer.apple.com/documentation/security/1643698-seckeycopyexternalrepresentation?language=objc.
  const char *pub_key_b64;

  // creation_date in ISO 8601 format.
  // Only present when reading existing credentials.
  const char *creation_date;
} CredentialInfo;

#endif // CREDENTIAL_INFO_H_
