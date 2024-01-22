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

export function ExpandIcon({ size = 16, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 16 16">
      <path d="m15 5-1.284-1.284-3.29 3.29-1.432-1.432 3.29-3.29L11 1a1 1 0 0 1 1-1h3a1 1 0 0 1 1 1v3a1 1 0 0 1-1 1zM5 15a1 1 0 0 1-1 1H1a1 1 0 0 1-1-1v-3a1 1 0 0 1 1-1l1.284 1.284 3.29-3.29 1.432 1.432-3.29 3.291L5 15z" />
    </SVGIcon>
  );
}
