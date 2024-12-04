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

import Box from './Box';

/**
 * TypeTest is supposed to catch regressions in the types for our design components.
 *
 * The file name specifically uses the "type-test.tsx" suffix so that this file isn't ran by Jest.
 */
export const TypeTest = () => (
  <Box>
    <Box
      // @ts-expect-error Prop that doesn't exist on Box.
      nonexistentprop
    />
    <Box
      // @ts-expect-error Valid prop but invalid value type.
      alignSelf={2}
    />
  </Box>
);
