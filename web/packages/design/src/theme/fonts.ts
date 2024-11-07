/*
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

import { getPlatformType } from '../platform';

const fontMonoLinux = `"Droid Sans Mono", "monospace", monospace, "Droid Sans Fallback"`;
const fontMonoWin = `Consolas, "Courier New", monospace`;
const fontMonoMac = `Menlo, Monaco, "Courier New", monospace`;

export const font = `Ubuntu2, -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol";`;

export interface Fonts {
  sansSerif: string;
  mono: string;
}

export const fonts: Fonts = {
  sansSerif: font,
  mono: getMonoFont(),
};

function getMonoFont() {
  const platform = getPlatformType();

  if (platform.isLinux) {
    return fontMonoLinux;
  }

  if (platform.isMac) {
    return fontMonoMac;
  }

  if (platform.isWin) {
    return fontMonoWin;
  }

  return fontMonoLinux;
}
