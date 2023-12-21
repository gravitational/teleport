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

export function SearchIcon({ size = 24, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 24 24">
      <path d="M10 9A1 1 0 1 0 10 11 1 1 0 1 0 10 9zM7 9A1 1 0 1 0 7 11 1 1 0 1 0 7 9zM13 9A1 1 0 1 0 13 11 1 1 0 1 0 13 9zM22 20L20 22 16 18 16 16 18 16z" />
      <path d="M10,18c-4.4,0-8-3.6-8-8c0-4.4,3.6-8,8-8c4.4,0,8,3.6,8,8C18,14.4,14.4,18,10,18z M10,4c-3.3,0-6,2.7-6,6c0,3.3,2.7,6,6,6 c3.3,0,6-2.7,6-6C16,6.7,13.3,4,10,4z" />
      <path
        d="M15.7 14.5H16.7V18H15.7z"
        transform="rotate(-45.001 16.25 16.249)"
      />
    </SVGIcon>
  );
}
