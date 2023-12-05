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

export function AzureIcon({ size = 80, fill }: SVGIconProps) {
  return (
    <SVGIcon size={size} fill={fill} viewBox="0 0 72 72">
      <path
        fill="url(#a)"
        d="M24.003 2.058h21.305l-22.117 65.53a3.397 3.397 0 0 1-3.218 2.312H3.392a3.392 3.392 0 0 1-3.214-4.476L20.784 4.369a3.398 3.398 0 0 1 3.219-2.311Z"
      />
      <path
        fill="#0078D4"
        d="M54.962 46.012H21.177a1.564 1.564 0 0 0-1.068 2.707l21.71 20.263c.632.59 1.464.918 2.328.918h19.13l-8.315-23.888Z"
      />
      <path
        fill="url(#b)"
        d="M24.002 2.058a3.37 3.37 0 0 0-3.226 2.356L.203 65.368A3.388 3.388 0 0 0 3.4 69.9H20.41a3.636 3.636 0 0 0 2.79-2.373l4.102-12.091 14.655 13.668a3.468 3.468 0 0 0 2.182.796h19.059l-8.36-23.888-24.367.006 14.914-43.96H24.002Z"
      />
      <path
        fill="url(#c)"
        d="M51.216 4.366A3.392 3.392 0 0 0 48 2.058H24.258a3.393 3.393 0 0 1 3.214 2.308l20.607 61.057a3.392 3.392 0 0 1-3.215 4.477H68.61a3.392 3.392 0 0 0 3.213-4.477L51.216 4.366Z"
      />
      <defs>
        <linearGradient
          id="a"
          x1="31.768"
          x2="9.642"
          y1="7.085"
          y2="72.452"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#114A8B" />
          <stop offset="1" stopColor="#0669BC" />
        </linearGradient>
        <linearGradient
          id="b"
          x1="38.679"
          x2="33.561"
          y1="37.548"
          y2="39.279"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopOpacity=".3" />
          <stop offset=".071" stopOpacity=".2" />
          <stop offset=".321" stopOpacity=".1" />
          <stop offset=".623" stopOpacity=".05" />
          <stop offset="1" stopOpacity="0" />
        </linearGradient>
        <linearGradient
          id="c"
          x1="35.865"
          x2="60.153"
          y1="5.179"
          y2="69.887"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#3CCBF4" />
          <stop offset="1" stopColor="#2892DF" />
        </linearGradient>
      </defs>
    </SVGIcon>
  );
}
