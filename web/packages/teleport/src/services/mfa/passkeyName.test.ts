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

import { composePasskeyName, uniquePasskeyName } from './passkeyName';

// Control the resolved name per case, so the tests cover this module rather than the generated spec map.
const mockResolveAuthenticatorName = jest.fn();

jest.mock('design/AuthenticatorIcon', () => ({
  resolveAuthenticatorName: (...args: unknown[]) =>
    mockResolveAuthenticatorName(...args),
}));

describe('composePasskeyName', () => {
  beforeEach(() => {
    mockResolveAuthenticatorName.mockReset();
  });

  it('uses the resolved authenticator name', () => {
    mockResolveAuthenticatorName.mockReturnValue('YubiKey 5 Series');
    const name = composePasskeyName(credential('cross-platform', ['usb']));

    expect(name).toBe('YubiKey 5 Series');
  });

  it('passes the AAGUID, transports and attachment to the resolver', () => {
    mockResolveAuthenticatorName.mockReturnValue('1Password');
    composePasskeyName(credential('platform', ['internal']));

    expect(mockResolveAuthenticatorName).toHaveBeenCalledWith(
      '',
      ['internal'],
      'platform'
    );
  });

  it('clips a name that exceeds the byte limit', () => {
    // 31 characters, one over the server's limit.
    mockResolveAuthenticatorName.mockReturnValue('A'.repeat(31));
    const name = composePasskeyName(credential('platform'));

    expect(name).toBe('A'.repeat(30));
  });

  it('clips on a character boundary for multibyte names', () => {
    // Ten 3-byte characters and then some: the limit falls mid-character.
    mockResolveAuthenticatorName.mockReturnValue('例'.repeat(11));
    const name = composePasskeyName(credential('platform'));

    expect(name).toBe('例'.repeat(10));
    expect(new TextEncoder().encode(name).length).toBe(30);
  });
});

describe('uniquePasskeyName', () => {
  it('returns the name unchanged when it is not taken', () => {
    expect(uniquePasskeyName('YubiKey 5 Series', ['Chrome on Mac'])).toBe(
      'YubiKey 5 Series'
    );
  });

  it('appends a counter when the name is taken', () => {
    expect(uniquePasskeyName('YubiKey 5 Series', ['YubiKey 5 Series'])).toBe(
      'YubiKey 5 Series (2)'
    );
  });

  it('skips counters that are also taken', () => {
    const taken = ['YubiKey 5 Series', 'YubiKey 5 Series (2)'];

    expect(uniquePasskeyName('YubiKey 5 Series', taken)).toBe(
      'YubiKey 5 Series (3)'
    );
  });

  it('compares case-insensitively and ignores surrounding whitespace', () => {
    expect(uniquePasskeyName('Chrome on Mac', ['  chrome on MAC '])).toBe(
      'Chrome on Mac (2)'
    );
  });

  it('trims the base name so the counter fits the length limit', () => {
    // Exactly 30 characters, the server's device name limit.
    const taken = 'YubiKey 5 Series with Lightnin';
    const name = uniquePasskeyName(taken, [taken]);

    expect(name).toBe('YubiKey 5 Series with Ligh (2)');
    expect(name).toHaveLength(30);
  });

  it('gives up and returns the plain name when every counter is taken', () => {
    const taken = ['Passkey'];
    for (let n = 2; n <= 100; n++) {
      taken.push(`Passkey (${n})`);
    }

    expect(uniquePasskeyName('Passkey', taken)).toBe('Passkey');
  });
});

function credential(attachment?: string, transports?: string[]) {
  return {
    authenticatorAttachment: attachment,
    response: {
      getAuthenticatorData: () => new Uint8Array(53).buffer,
      getTransports: () => transports,
    },
  } as unknown as Credential;
}
