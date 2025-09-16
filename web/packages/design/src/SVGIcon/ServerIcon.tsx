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

import React from 'react';

import type { SVGIconProps } from './common';
import { SVGIcon } from './SVGIcon';

export function ServerIcon({ size = 48, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 48 48">
      <path d="M 9.5 6 C 7.0324991 6 5 8.0324991 5 10.5 L 5 37.5 C 5 39.967501 7.0324991 42 9.5 42 L 17.5 42 C 19.967501 42 22 39.967501 22 37.5 L 22 25.5 L 24 25.5 L 24 32.5 C 24 34.414955 25.585045 36 27.5 36 L 29 36 L 29 37.5 C 29 39.967501 31.032499 42 33.5 42 L 38.5 42 C 40.967501 42 43 39.967501 43 37.5 L 43 29.5 C 43 27.032499 40.967501 25 38.5 25 L 33.5 25 C 31.032499 25 29 27.032499 29 29.5 L 29 33 L 27.5 33 C 27.204955 33 27 32.795045 27 32.5 L 27 24.246094 A 1.50015 1.50015 0 0 0 27 23.759766 L 27 15.5 C 27 15.204955 27.204955 15 27.5 15 L 29 15 L 29 18.5 C 29 20.967501 31.032499 23 33.5 23 L 38.5 23 C 40.967501 23 43 20.967501 43 18.5 L 43 10.5 C 43 8.0324991 40.967501 6 38.5 6 L 33.5 6 C 31.032499 6 29 8.0324991 29 10.5 L 29 12 L 27.5 12 C 25.585045 12 24 13.585045 24 15.5 L 24 22.5 L 22 22.5 L 22 10.5 C 22 8.0324991 19.967501 6 17.5 6 L 9.5 6 z M 9.5 9 L 17.5 9 C 18.346499 9 19 9.6535009 19 10.5 L 19 23.753906 A 1.50015 1.50015 0 0 0 19 24.240234 L 19 37.5 C 19 38.346499 18.346499 39 17.5 39 L 9.5 39 C 8.6535009 39 8 38.346499 8 37.5 L 8 10.5 C 8 9.6535009 8.6535009 9 9.5 9 z M 33.5 9 L 38.5 9 C 39.346499 9 40 9.6535009 40 10.5 L 40 18.5 C 40 19.346499 39.346499 20 38.5 20 L 33.5 20 C 32.653501 20 32 19.346499 32 18.5 L 32 13.746094 A 1.50015 1.50015 0 0 0 32 13.259766 L 32 10.5 C 32 9.6535009 32.653501 9 33.5 9 z M 11.5 12 A 1.50015 1.50015 0 1 0 11.5 15 L 15.5 15 A 1.50015 1.50015 0 1 0 15.5 12 L 11.5 12 z M 36 15 A 1.5 1.5 0 0 0 36 18 A 1.5 1.5 0 0 0 36 15 z M 11.5 17 A 1.50015 1.50015 0 1 0 11.5 20 L 15.5 20 A 1.50015 1.50015 0 1 0 15.5 17 L 11.5 17 z M 33.5 28 L 38.5 28 C 39.346499 28 40 28.653501 40 29.5 L 40 37.5 C 40 38.346499 39.346499 39 38.5 39 L 33.5 39 C 32.653501 39 32 38.346499 32 37.5 L 32 34.746094 A 1.50015 1.50015 0 0 0 32 34.259766 L 32 29.5 C 32 28.653501 32.653501 28 33.5 28 z M 13.5 33 A 1.5 1.5 0 0 0 13.5 36 A 1.5 1.5 0 0 0 13.5 33 z M 36 34 A 1.5 1.5 0 0 0 36 37 A 1.5 1.5 0 0 0 36 34 z" />
    </SVGIcon>
  );
}
