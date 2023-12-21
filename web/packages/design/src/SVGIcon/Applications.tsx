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

export function ApplicationsIcon({ size = 22, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 20 22" size={size} fill={fill}>
      <path d="M18.125 0.5H1.875C0.839844 0.5 0 1.50781 0 2.75V19.25C0 20.4922 0.839844 21.5 1.875 21.5H18.125C19.1602 21.5 20 20.4922 20 19.25V2.75C20 1.50781 19.1602 0.5 18.125 0.5ZM1.25 2.75C1.25 2.3375 1.53125 2 1.875 2H3.75V5H1.25V2.75ZM18.75 19.25C18.75 19.6625 18.4688 20 18.125 20H1.875C1.53125 20 1.25 19.6625 1.25 19.25V6.5H18.75V19.25ZM18.75 5H5V2H18.125C18.4688 2 18.75 2.3375 18.75 2.75V5Z" />
    </SVGIcon>
  );
}
