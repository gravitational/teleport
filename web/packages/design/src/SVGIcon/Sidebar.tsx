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

export function SidebarIcon({ size = 24, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 24 24">
      <path d="M5 2a2.997 2.997 0 0 0-3 3v14a2.997 2.997 0 0 0 3 3h14a2.997 2.997 0 0 0 3-3V5a2.997 2.997 0 0 0-3-3zm5 18V4h9c.276 0 .525.111.707.293S20 4.724 20 5v14c0 .276-.111.525-.293.707S19.276 20 19 20zM8 4v16H5c-.276 0-.525-.111-.707-.293S4 19.276 4 19V5c0-.276.111-.525.293-.707S4.724 4 5 4z" />
    </SVGIcon>
  );
}
