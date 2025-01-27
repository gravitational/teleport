/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { z } from 'zod';

import Logger from 'teleterm/logger';

export type ProfileColor = z.infer<typeof profileColors>;

export const profileColors = z.enum([
  'purple',
  'green',
  'yellow',
  'red',
  'cyan',
  'pink',
  'blue',
]);

// TODO(gzdunek): Parse the entire workspace state read from disk like below.
export function parseProfileColor(
  color: unknown,
  workspaces: Record<
    string,
    {
      color: ProfileColor;
    }
  >
): ProfileColor {
  const getDefault = () => getNextProfileColor(workspaces);
  return profileColors
    .default(getDefault)
    .catch(ctx => {
      new Logger('WorkspacesService').error(
        'Failed to read profile color preference',
        ctx.error
      );
      return getDefault();
    })
    .parse(color);
}

/**
 * Determines the next available unused color across all workspaces.
 * If all colors are already in use, it defaults to returning purple.
 */
function getNextProfileColor(
  workspaces: Record<string, { color: ProfileColor }>
): ProfileColor {
  const takenColors = new Set(Object.values(workspaces).map(w => w.color));
  const allColors = new Set(profileColors.options);
  const unusedColors = allColors.difference(takenColors);
  return unusedColors.size > 0 ? [...unusedColors][0] : 'purple';
}
