/**
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

// WebAuthn authenticator data is a 32-byte RP ID hash, a flags byte, a 4-byte signature
// counter, and then the attested credential data, which opens with the AAGUID.
const FLAGS_OFFSET = 32;
const AAGUID_OFFSET = 37;
const AAGUID_LENGTH = 16;
const MIN_AUTH_DATA_LENGTH = AAGUID_OFFSET + AAGUID_LENGTH;

// Bit 6 of the flags marks attested credential data. Without it, the bytes at
// AAGUID_OFFSET are extension data rather than the AAGUID.
const FLAG_ATTESTED_CREDENTIAL_DATA = 1 << 6;

const HEX_DIGITS_PER_BYTE = 2;

// A dashed UUID groups its 16 bytes as 4-2-2-2-6.
const UUID_BYTE_GROUPS = [4, 2, 2, 2, 6];

// aaguidFromCredential extracts the authenticator AAGUID from a freshly created credential's
// authenticator data, formatted as a dashed UUID.
//
// Returns '' when unavailable (older browsers) or all-zero.
export function aaguidFromCredential(cred: Credential) {
  const resp = (cred as PublicKeyCredential)
    ?.response as AuthenticatorAttestationResponse;

  if (!resp?.getAuthenticatorData) {
    return '';
  }

  const authData = new Uint8Array(resp.getAuthenticatorData());
  if (authData.length < MIN_AUTH_DATA_LENGTH) {
    return '';
  }

  if (!(authData[FLAGS_OFFSET] & FLAG_ATTESTED_CREDENTIAL_DATA)) {
    return '';
  }

  const bytes = authData.slice(AAGUID_OFFSET, AAGUID_OFFSET + AAGUID_LENGTH);
  if (bytes.every(b => b === 0)) {
    return '';
  }

  return toDashedUuid(bytes);
}

function toDashedUuid(bytes: Uint8Array) {
  const hex = Array.from(bytes).map(b =>
    b.toString(16).padStart(HEX_DIGITS_PER_BYTE, '0')
  );

  const groups: string[] = [];

  let offset = 0;
  for (const length of UUID_BYTE_GROUPS) {
    groups.push(hex.slice(offset, offset + length).join(''));

    offset += length;
  }

  return groups.join('-');
}

// transportsOf returns the WebAuthn transports reported for a freshly created credential,
// or an empty list when the browser does not expose them.
export function transportsOf(cred: Credential) {
  const resp = (cred as PublicKeyCredential)
    ?.response as AuthenticatorAttestationResponse;

  return resp?.getTransports?.() ?? [];
}
