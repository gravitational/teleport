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

export const Memory = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-memory"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M14.25 1.5a.75.75 0 0 1 .75.75v1.5h3.75a1.5 1.5 0 0 1 1.5 1.5V9h1.5a.75.75 0 0 1 0 1.5h-1.5v3h1.5a.75.75 0 0 1 0 1.5h-1.5v3.75a1.5 1.5 0 0 1-1.5 1.5H15v1.5a.75.75 0 0 1-1.5 0v-1.5h-3v1.5a.75.75 0 0 1-1.5 0v-1.5H5.25a1.5 1.5 0 0 1-1.5-1.5V15h-1.5a.75.75 0 0 1 0-1.5h1.5v-3h-1.5a.75.75 0 0 1 0-1.5h1.5V5.25a1.5 1.5 0 0 1 1.5-1.5H9v-1.5a.75.75 0 0 1 1.5 0v1.5h3v-1.5a.75.75 0 0 1 .75-.75m-4.5 3.75h-4.5v13.5h13.5V5.25zm0 3.75a.75.75 0 0 0-.75.75v4.5c0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75v-4.5a.75.75 0 0 0-.75-.75zm.75 4.5v-3h3v3z"
        clipRule="evenodd"
      />
    </Icon>
  )
);
