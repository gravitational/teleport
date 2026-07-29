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

import { aaguidFromCredential, transportsOf } from './aaguid';

describe('aaguidFromAttestationResponse', () => {
  it('extracts a dashed uuid from bytes 37..53', () => {
    // ee882879-721c-4913-9775-3dfcce97072a
    const aaguid = [
      0xee, 0x88, 0x28, 0x79, 0x72, 0x1c, 0x49, 0x13, 0x97, 0x75, 0x3d, 0xfc,
      0xce, 0x97, 0x07, 0x2a,
    ];
    const cred = credentialWithAuthData(authDataWithAaguid(aaguid));

    expect(aaguidFromCredential(cred)).toBe(
      'ee882879-721c-4913-9775-3dfcce97072a'
    );
  });

  it('returns empty string for an all-zero AAGUID', () => {
    const cred = credentialWithAuthData(
      authDataWithAaguid(new Array(16).fill(0))
    );

    expect(aaguidFromCredential(cred)).toBe('');
  });

  it('returns empty string when getAuthenticatorData is missing', () => {
    const cred = credentialWithAuthData(undefined);

    expect(aaguidFromCredential(cred)).toBe('');
  });

  it('returns empty string when authenticator data is too short', () => {
    const cred = credentialWithAuthData(new Uint8Array(40));

    expect(aaguidFromCredential(cred)).toBe('');
  });

  it('returns empty string when attested credential data is absent', () => {
    // Without the flag, bytes 37 onwards are extension data, not an AAGUID.
    const aaguid = new Array(16).fill(0xab);
    const cred = credentialWithAuthData(authDataWithAaguid(aaguid, 53, false));

    expect(aaguidFromCredential(cred)).toBe('');
  });

  it('returns empty string for a null credential', () => {
    expect(aaguidFromCredential(null)).toBe('');
  });
});

describe('transportsOf', () => {
  it('returns the reported transports', () => {
    const cred = {
      response: { getTransports: () => ['usb', 'nfc'] },
    } as unknown as Credential;

    expect(transportsOf(cred)).toEqual(['usb', 'nfc']);
  });

  it('returns an empty list when getTransports is unavailable', () => {
    const cred = { response: {} } as unknown as Credential;

    expect(transportsOf(cred)).toEqual([]);
  });
});

// Builds authenticator data of the given length with the 16 AAGUID bytes placed at offset
// 37, matching the WebAuthn wire layout the extractor slices from.
// Byte 32 holds the flags; bit 6 marks the attested credential data that carries the AAGUID.
function authDataWithAaguid(aaguid: number[], length = 53, attested = true) {
  const data = new Uint8Array(length);
  if (attested) {
    data[32] = 1 << 6;
  }

  data.set(aaguid, 37);

  return data;
}

function credentialWithAuthData(data?: Uint8Array) {
  return {
    response: {
      getAuthenticatorData: data ? () => data.buffer : undefined,
    },
  } as unknown as Credential;
}
