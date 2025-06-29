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

/* MIT License

Copyright (c) 2020 Phosphor Icons

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/

import { forwardRef } from 'react';

import { Icon, IconProps } from '../Icon';

/*

THIS FILE IS GENERATED. DO NOT EDIT.

*/

export const Devices = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-devices"
      {...otherProps}
      ref={ref}
    >
      <path d="M18 9.75a.75.75 0 0 0 0 1.5h1.5a.75.75 0 0 0 0-1.5z" />
      <path
        fillRule="evenodd"
        d="M3.75 17.25h10.5V18a2.25 2.25 0 0 0 2.25 2.25H21A2.25 2.25 0 0 0 23.25 18V9A2.25 2.25 0 0 0 21 6.75h-1.5V6a2.25 2.25 0 0 0-2.25-2.25H3.75A2.25 2.25 0 0 0 1.5 6v9a2.25 2.25 0 0 0 2.25 2.25m0-12A.75.75 0 0 0 3 6v9a.75.75 0 0 0 .75.75h10.5V9a2.25 2.25 0 0 1 2.25-2.25H18V6a.75.75 0 0 0-.75-.75zm17.25 3h-4.5a.75.75 0 0 0-.75.75v9c0 .414.336.75.75.75H21a.75.75 0 0 0 .75-.75V9a.75.75 0 0 0-.75-.75"
        clipRule="evenodd"
      />
      <path d="M8.25 18.75a.75.75 0 0 0 0 1.5H12a.75.75 0 0 0 0-1.5z" />
    </Icon>
  )
);
