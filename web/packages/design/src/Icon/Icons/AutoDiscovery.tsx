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

import { forwardRef } from 'react';

import { Icon, IconProps } from '../Icon';

/*

THIS FILE IS GENERATED. DO NOT EDIT.

*/

export const AutoDiscovery = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-autodiscovery"
      {...otherProps}
      ref={ref}
    >
      <path d="M18 6.5C18.4142 6.5 18.75 6.16421 18.75 5.75C18.75 5.33579 18.4142 5 18 5C17.5858 5 17.25 5.33579 17.25 5.75C17.25 6.16421 17.5858 6.5 18 6.5Z" fill="currentColor"/>
      <path fillRule="evenodd" clipRule="evenodd" d="M4 2C3.17157 2 2.5 2.67157 2.5 3.5V7.5C2.5 8.32843 3.17157 9 4 9H20C20.8284 9 21.5 8.32843 21.5 7.5V3.5C21.5 2.67157 20.8284 2 20 2H4ZM4 3.5H20V7.5H4V3.5Z" fill="currentColor"/>
      <path d="M18.75 13.25C18.75 13.6642 18.4142 14 18 14C17.5858 14 17.25 13.6642 17.25 13.25C17.25 12.8358 17.5858 12.5 18 12.5C18.4142 12.5 18.75 12.8358 18.75 13.25Z" fill="currentColor"/>
      <path fillRule="evenodd" clipRule="evenodd" d="M4 10.5C3.17157 10.5 2.5 11.1716 2.5 12V16C2.5 16.8284 3.17157 17.5 4 17.5H20C20.8284 17.5 21.5 16.8284 21.5 16V12C21.5 11.1716 20.8284 10.5 20 10.5H4ZM4 12H20V16H4V12Z" fill="currentColor"/>
      <g transform="translate(12, 9)">
        <circle cx="5" cy="5" r="4" stroke="currentColor" strokeWidth="1.2" fill="white"/>
        <path d="M7.828 7.828L10.5 10.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
      </g>
    </Icon>
  )
);
