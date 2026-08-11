/**
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

import { sharedStyles } from './sharedStyles';
import { type ThemeDefinition } from './types';

// Color tokens live in the design-system package now, e.g. for bblp:
// https://github.com/gravitational/design-system/blob/main/src/themes/bblp/colors.ts
// Only legacy styled-components shape lives here.
const theme: ThemeDefinition = {
  ...sharedStyles,
  name: 'bblp',
  type: 'dark',
  isCustomTheme: true,
};

export default theme;
