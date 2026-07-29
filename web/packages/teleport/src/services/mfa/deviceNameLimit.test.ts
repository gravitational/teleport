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

import { authenticatorSpecs } from 'design/AuthenticatorIcon/authenticatorSpecs';

import { MAX_DEVICE_NAME_BYTES } from './passkeyName';

// The generator clips vendor names to its own copy of the server's limit (MAX_NAME_BYTES
// in AuthenticatorIcon/script/generate.mjs), and those names are offered verbatim as
// device nicknames.
// A name over budget would only be rejected once the user had finished the ceremony, so
// hold the generated data to the limit the app knows about.
test('every generated authenticator name fits the server device name limit', () => {
  const encoder = new TextEncoder();

  const tooLong = Object.entries(authenticatorSpecs)
    .map(([aaguid, spec]) => ({
      aaguid,
      name: spec.name,
      bytes: encoder.encode(spec.name).length,
    }))
    .filter(entry => entry.bytes > MAX_DEVICE_NAME_BYTES);

  expect(tooLong).toEqual([]);
});
