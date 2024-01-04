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

import { SVGIcon } from './SVGIcon';

import type { SVGIconProps } from './common';

export function UpgradeIcon({ size = 50, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 50 50">
      <path d="M 20.011719 2 A 1.0001 1.0001 0 0 0 19.654297 2.0625 L 0.65429688 9.0625 A 1.0001 1.0001 0 0 0 0.15234375 10.529297 L 4.8222656 18 L 0.15234375 25.470703 A 1.0001 1.0001 0 0 0 0.65429688 26.9375 L 5 28.539062 L 5 40 A 1.0001 1.0001 0 0 0 5.6542969 40.9375 L 24.654297 47.9375 A 1.0001 1.0001 0 0 0 25.345703 47.9375 L 44.345703 40.9375 A 1.0001 1.0001 0 0 0 45 40 L 45 18.285156 L 49.847656 10.529297 A 1.0001 1.0001 0 0 0 49.345703 9.0625 L 30.345703 2.0625 A 1.0001 1.0001 0 0 0 29.953125 2.0019531 A 1.0001 1.0001 0 0 0 29.152344 2.4707031 L 25 9.1152344 L 20.847656 2.4707031 A 1.0001 1.0001 0 0 0 20.011719 2 z M 19.582031 4.21875 L 23.501953 10.488281 L 6.4160156 16.78125 L 2.4980469 10.511719 L 19.582031 4.21875 z M 30.417969 4.21875 L 46.111328 10 L 35.642578 13.855469 L 26.5 10.486328 L 30.417969 4.21875 z M 46.605469 11.947266 L 43.333984 17.179688 L 27.394531 23.052734 L 30.666016 17.820312 L 46.605469 11.947266 z M 25 12.066406 L 32.751953 14.921875 L 29.654297 16.0625 A 1.0001 1.0001 0 0 0 29.152344 16.470703 L 24.583984 23.78125 L 8.890625 18 L 25 12.066406 z M 6.4179688 19.21875 L 23.5 25.513672 L 19.583984 31.78125 L 2.4980469 25.488281 L 6.4179688 19.21875 z M 43 19.433594 L 43 39.302734 L 26 45.564453 L 26 25.697266 L 43 19.433594 z M 24 28.486328 L 24 45.564453 L 7 39.302734 L 7 29.275391 L 19.654297 33.9375 A 1.0001 1.0001 0 0 0 20.847656 33.529297 L 24 28.486328 z" />
    </SVGIcon>
  );
}
