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

export function RunIcon({ size = 48, fill = 'white' }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 64 64">
      <path d="M26.023,43.745C25.467,43.283,25,42.724,25,42V23c0-0.734,0.402-1.41,1.048-1.759s1.431-0.316,2.046,0.084l15,9.798	c0.574,0.375,0.916,1.018,0.906,1.703c-0.01,0.686-0.37,1.318-0.954,1.676l-15,9.202C27.726,43.901,26.58,44.207,26.023,43.745z M29,26.695v11.731l9.262-5.682L29,26.695z" />
      <path d="M32,54c-12.131,0-22-9.869-22-22s9.869-22,22-22s22,9.869,22,22S44.131,54,32,54z M32,14	c-9.925,0-18,8.075-18,18s8.075,18,18,18s18-8.075,18-18S41.925,14,32,14z" />
    </SVGIcon>
  );
}
