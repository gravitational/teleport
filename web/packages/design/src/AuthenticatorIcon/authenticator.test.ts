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

import { authenticatorName, resolveAuthenticatorName } from './authenticator';

// The generated spec map pulls in hundreds of image assets. These tests exercise the
// resolver logic with a hand-built map instead, so stub the generated module out of the
// graph entirely.

jest.mock('./authenticatorIcons', () => ({ authenticatorIcons: {} }));

const ZERO_AAGUID = '00000000-0000-0000-0000-000000000000';

// Names in the generated spec are already normalized by the generator: parentheticals
// dropped and clipped to 30 characters. Model that here so the resolver is tested against
// realistic values.
const clipped = 'SECORA ID Key S USB by Infineo'; // exactly 30 chars

const specs: Record<string, string> = {
  'ee882879-721c-4913-9775-3dfcce97072a': 'YubiKey 5 Series',
  'aaaaaaaa-0000-0000-0000-000000000000': clipped,
};

describe('resolveAuthenticatorName', () => {
  it('returns the normalized name for a known AAGUID', () => {
    expect(
      resolveAuthenticatorName(
        'ee882879-721c-4913-9775-3dfcce97072a',
        [],
        '',
        specs
      )
    ).toBe('YubiKey 5 Series');
  });

  it('returns the clipped name verbatim for a known AAGUID', () => {
    const name = resolveAuthenticatorName(
      'aaaaaaaa-0000-0000-0000-000000000000',
      [],
      '',
      specs
    );

    expect(name).toBe(clipped);
    expect(name).toHaveLength(30);
  });

  it('prefers a known AAGUID over transport/attachment fallbacks', () => {
    expect(
      resolveAuthenticatorName(
        'ee882879-721c-4913-9775-3dfcce97072a',
        ['usb'],
        'cross-platform',
        specs
      )
    ).toBe('YubiKey 5 Series');
  });

  describe('falls back when the AAGUID is unknown or all-zero', () => {
    it.each([
      {
        attachment: 'platform',
        transports: [],
        want: 'Passkey on this device',
      },
      {
        attachment: '',
        transports: ['internal'],
        want: 'Passkey on this device',
      },
      { attachment: '', transports: ['hybrid'], want: 'Phone passkey' },
      {
        attachment: 'cross-platform',
        transports: [],
        want: 'Security key',
      },
      { attachment: '', transports: ['usb'], want: 'Security key' },
      { attachment: '', transports: ['nfc'], want: 'Security key' },
      { attachment: '', transports: ['ble'], want: 'Security key' },
      { attachment: '', transports: [], want: 'Passkey' },
    ])(
      'attachment=$attachment transports=$transports -> $want',
      ({ attachment, transports, want }) => {
        // Unknown AAGUID.
        expect(
          resolveAuthenticatorName(
            'unknown-aaguid',
            transports,
            attachment,
            specs
          )
        ).toBe(want);

        // All-zero AAGUID is treated as absent even when present in the map.
        expect(
          resolveAuthenticatorName(ZERO_AAGUID, transports, attachment, {
            ...specs,
            [ZERO_AAGUID]: 'should be ignored',
          })
        ).toBe(want);
      }
    );
  });

  it('prefers platform over hybrid when both are indicated', () => {
    expect(
      resolveAuthenticatorName(
        undefined,
        ['hybrid', 'internal'],
        'platform',
        specs
      )
    ).toBe('Passkey on this device');
  });

  it('falls back to Passkey when the AAGUID is undefined and no hints are given', () => {
    expect(resolveAuthenticatorName(undefined)).toBe('Passkey');
  });

  it('ignores AAGUIDs naming inherited properties', () => {
    // specs['constructor'] would otherwise resolve to the Object constructor, named "Object".
    expect(resolveAuthenticatorName('constructor', [], '', specs)).toBe(
      'Passkey'
    );
    expect(resolveAuthenticatorName('__proto__', ['usb'], '', specs)).toBe(
      'Security key'
    );
  });
});

describe('authenticatorName', () => {
  it('returns the vendor name for a known AAGUID', () => {
    expect(
      authenticatorName('ee882879-721c-4913-9775-3dfcce97072a', specs)
    ).toBe('YubiKey 5 Series');
  });

  it.each(['unknown-aaguid', ZERO_AAGUID, 'constructor', undefined])(
    'returns undefined for %s',
    aaguid => {
      expect(authenticatorName(aaguid, specs)).toBeUndefined();
    }
  );
});
