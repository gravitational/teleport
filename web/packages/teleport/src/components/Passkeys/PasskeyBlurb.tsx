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

import Box from 'design/Box';

import { PasskeyIcons } from 'teleport/components/Passkeys';

export function PasskeyBlurb() {
  return (
    <Box
      border={1}
      borderColor="interactive.tonal.neutral.2"
      borderRadius={3}
      p={3}
    >
      <PasskeyIcons />
      <p>
        Teleport supports passkeys, a password replacement that validates your
        identity using touch, facial recognition, a device password, or a PIN.
      </p>
      <p style={{ marginBottom: 0 }}>
        Passkeys can be used to sign in as a simple and secure alternative to
        your password and multi-factor credentials.
      </p>
    </Box>
  );
}
