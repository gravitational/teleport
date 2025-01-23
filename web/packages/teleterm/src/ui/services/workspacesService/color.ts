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

import { dataVisualisationColors } from 'design/theme/themes/darkTheme';

import Logger from 'teleterm/logger';

export type WorkspaceColor = z.infer<typeof workspaceColors>;

export const workspaceColors = z.enum([
  'purple',
  'green',
  'yellow',
  'red',
  'cyan',
  'pink',
  'blue',
]);

// TODO(gzdunek): Parse the entire workspace state read from disk like below.
export function parseWorkspaceColor(
  color: unknown,
  workspaces: Record<
    string,
    {
      color: WorkspaceColor;
    }
  >
): WorkspaceColor {
  const getDefault = () => getNextWorkspaceColor(workspaces);
  return workspaceColors
    .default(getDefault)
    .catch(ctx => {
      new Logger('WorkspacesService').error(
        'Failed to read workspace color preference',
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
function getNextWorkspaceColor(
  workspaces: Record<string, { color: WorkspaceColor }>
): WorkspaceColor {
  const takenColors = new Set(Object.values(workspaces).map(w => w.color));
  const allColors = new Set(workspaceColors.options);
  const unusedColors = allColors.difference(takenColors);
  return unusedColors.size > 0 ? [...unusedColors][0] : 'purple';
}

/**
 * Maps profile color code to the theme color.
 * We always use dark theme colors.
 * They look good in both light and dark modes,
 * and we avoid confusing users with different shades of the same color.
 */
export const profileColorMapping: Record<ProfileColor, string> = {
  purple: dataVisualisationColors.primary.purple,
  red: dataVisualisationColors.primary.abbey,
  green: dataVisualisationColors.primary.caribbean,
  yellow: dataVisualisationColors.primary.sunflower,
  blue: dataVisualisationColors.primary.picton,
  cyan: dataVisualisationColors.primary.cyan,
  pink: dataVisualisationColors.primary.wednesdays,
};
