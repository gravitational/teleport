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

export function ChevronRightIcon({ size = 14, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 9 14" size={size} fill={fill}>
      <path d="M1.65386 0.111133L1.02792 0.737077C0.879771 0.885224 0.879771 1.12543 1.02792 1.27361L6.7407 7L1.02795 12.7264C0.8798 12.8746 0.8798 13.1148 1.02795 13.2629L1.65389 13.8889C1.80204 14.037 2.04225 14.037 2.19043 13.8889L8.81101 7.26825C8.95915 7.1201 8.95915 6.87989 8.81101 6.73171L2.1904 0.111133C2.04222 -0.0370463 1.80201 -0.0370464 1.65386 0.111133Z" />
    </SVGIcon>
  );
}
