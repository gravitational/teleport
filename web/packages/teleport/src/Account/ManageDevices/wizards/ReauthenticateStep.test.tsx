/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { createReauthOptions } from './ReauthenticateStep';
import { deviceCases } from './deviceCases';

test.each`
  auth2faType   | deviceCase      | methods
  ${'otp'}      | ${'all'}        | ${['otp']}
  ${'off'}      | ${'all'}        | ${[]}
  ${'optional'} | ${'all'}        | ${['webauthn', 'otp']}
  ${'on'}       | ${'all'}        | ${['webauthn', 'otp']}
  ${'webauthn'} | ${'all'}        | ${['webauthn']}
  ${'optional'} | ${'authApps'}   | ${['otp']}
  ${'optional'} | ${'mfaDevices'} | ${['webauthn']}
  ${'optional'} | ${'passkeys'}   | ${['webauthn']}
  ${'on'}       | ${'none'}       | ${[]}
  ${'webauthn'} | ${'authApps'}   | ${[]}
  ${'otp'}      | ${'mfaDevices'} | ${[]}
`(
  'createReauthOptions: auth2faType=$auth2faType, devices=$deviceCase',
  ({ auth2faType, methods, deviceCase }) => {
    const devices = deviceCases[deviceCase];
    const reauthMethods = createReauthOptions(auth2faType, devices).map(
      o => o.value
    );
    expect(reauthMethods).toEqual(methods);
  }
);
