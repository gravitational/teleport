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

export function DesktopsIcon({ size = 20, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 20 19" size={size} fill={fill}>
      <path d="M18.3333 0.833313H1.66667C0.746528 0.833313 0 1.57984 0 2.49998V12.5C0 13.4201 0.746528 14.1666 1.66667 14.1666H8.33333L7.5 17.5H5C4.69444 17.5 4.44444 17.75 4.44444 18.0555C4.44444 18.3611 4.69444 18.6111 5 18.6111H15C15.3056 18.6111 15.5556 18.3611 15.5556 18.0555C15.5556 17.75 15.3056 17.5 15 17.5H12.5L11.6667 14.1666H18.3333C19.2535 14.1666 20 13.4201 20 12.5V2.49998C20 1.57984 19.2535 0.833313 18.3333 0.833313ZM8.64583 17.5L9.20139 15.2778H10.7986L11.3542 17.5H8.64583ZM18.8889 12.5C18.8889 12.8055 18.6389 13.0555 18.3333 13.0555H1.66667C1.36111 13.0555 1.11111 12.8055 1.11111 12.5V2.49998C1.11111 2.19442 1.36111 1.94442 1.66667 1.94442H18.3333C18.6389 1.94442 18.8889 2.19442 18.8889 2.49998V12.5Z" />
    </SVGIcon>
  );
}
