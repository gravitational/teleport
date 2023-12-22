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

export function AddIcon({ size = 10, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 10 10" size={size} fill={fill}>
      <path d="M9.07388 5.574H5.57388V9.074H4.42529V5.574H0.925293V4.42542H4.42529V0.925415H5.57388V4.42542H9.07388V5.574Z" />
    </SVGIcon>
  );
}
