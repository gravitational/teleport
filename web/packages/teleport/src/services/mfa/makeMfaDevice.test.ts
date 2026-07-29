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

import makeMfaDevice from './makeMfaDevice';

describe('makeMfaDevice', () => {
  it('maps aaguid onto the returned device', () => {
    const d = makeMfaDevice(
      {
        id: '1',
        name: 'yubikey',
        type: 'WebAuthn',
        addedAt: '2024-01-01T00:00:00Z',
        lastUsed: '2024-01-02T00:00:00Z',
        residentKey: true,
        aaguid: 'ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4',
      },
      {}
    );
    expect(d.aaguid).toBe('ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4');
  });

  it('leaves aaguid undefined when absent from json', () => {
    const d = makeMfaDevice(
      {
        id: '1',
        name: 'authenticator app',
        type: 'TOTP',
        addedAt: '2024-01-01T00:00:00Z',
        lastUsed: '2024-01-02T00:00:00Z',
      },
      {}
    );
    expect(d.aaguid).toBeUndefined();
  });
});
