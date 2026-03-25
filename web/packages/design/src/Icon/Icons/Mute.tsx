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

export const Mute = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-mute"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M14.58 2.326A.75.75 0 0 1 15 3v7.015a.75.75 0 0 1-1.5 0V4.533l-2.526 1.964a.75.75 0 0 1-.92-1.184l3.736-2.905a.75.75 0 0 1 .79-.082m-9.525.92a.75.75 0 0 0-1.11 1.009L6.895 7.5H3A1.5 1.5 0 0 0 1.5 9v6A1.5 1.5 0 0 0 3 16.5h4.243l6.547 5.092A.75.75 0 0 0 15 21v-4.585l3.945 4.34a.75.75 0 0 0 1.11-1.01l-5.241-5.765-.018-.02-6.089-6.697-.027-.03zM6.75 9H3v6h3.75zm1.5 6.383 5.25 4.083v-4.701L8.25 8.99zm9.254-5.926a.75.75 0 0 1 1.059.067 3.75 3.75 0 0 1 0 4.957.75.75 0 1 1-1.126-.992 2.25 2.25 0 0 0 0-2.974.75.75 0 0 1 .067-1.058M21.34 7a.75.75 0 1 0-1.117 1 6 6 0 0 1 0 8 .75.75 0 1 0 1.117 1 7.5 7.5 0 0 0 0-10"
        clipRule="evenodd"
      />
    </Icon>
  )
);
