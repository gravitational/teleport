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

export const Printer = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-printer"
      {...otherProps}
      ref={ref}
    >
      <path d="M18.563 10.875a.937.937 0 1 1-1.875 0 .937.937 0 0 1 1.875 0" />
      <path
        fillRule="evenodd"
        d="M6 3a.75.75 0 0 0-.75.75v3H3.875C2.62 6.75 1.5 7.702 1.5 9v7.5c0 .414.336.75.75.75h3v3c0 .414.336.75.75.75h12a.75.75 0 0 0 .75-.75v-3h3a.75.75 0 0 0 .75-.75V9c0-1.298-1.12-2.25-2.375-2.25H18.75v-3A.75.75 0 0 0 18 3zM3 9c0-.36.334-.75.875-.75h16.25c.54 0 .875.39.875.75v6.75h-2.25v-1.5a.75.75 0 0 0-.75-.75H6a.75.75 0 0 0-.75.75v1.5H3zm3.75-2.25h10.5V4.5H6.75zm0 8.25h10.5v4.5H6.75z"
        clipRule="evenodd"
      />
    </Icon>
  )
);
