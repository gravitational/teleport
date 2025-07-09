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

export const Calendar = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-calendar"
      {...otherProps}
      ref={ref}
    >
      <path d="M10.144 10.612a.75.75 0 0 1 .356.638v6a.75.75 0 0 1-1.5 0v-4.786l-.415.207a.75.75 0 1 1-.67-1.342l1.5-.75a.75.75 0 0 1 .73.033m3.932 1.411a.75.75 0 0 1 .77 1.18L12.15 16.8a.75.75 0 0 0 .6 1.2h3a.75.75 0 0 0 0-1.5h-1.5l1.798-2.398a2.25 2.25 0 1 0-3.746-2.48.75.75 0 1 0 1.297.754.75.75 0 0 1 .478-.353" />
      <path
        fillRule="evenodd"
        d="M17.25 2.25a.75.75 0 0 0-1.5 0V3h-7.5v-.75a.75.75 0 0 0-1.5 0V3H4.5A1.5 1.5 0 0 0 3 4.5v15A1.5 1.5 0 0 0 4.5 21h15a1.5 1.5 0 0 0 1.5-1.5v-15A1.5 1.5 0 0 0 19.5 3h-2.25zM19.5 4.5h-2.25v.75a.75.75 0 0 1-1.5 0V4.5h-7.5v.75a.75.75 0 0 1-1.5 0V4.5H4.5v3h15zM4.5 9h15v10.5h-15z"
        clipRule="evenodd"
      />
    </Icon>
  )
);
