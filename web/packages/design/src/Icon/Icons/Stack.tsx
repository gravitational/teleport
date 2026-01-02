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

export const Stack = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-stack"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M12.378 1.602a.75.75 0 0 0-.756 0l-9 5.25a.75.75 0 0 0 0 1.296l9 5.25a.75.75 0 0 0 .756 0l9-5.25a.75.75 0 0 0 0-1.296zM12 11.882 4.488 7.5 12 3.118 19.512 7.5z"
        clipRule="evenodd"
      />
      <path d="M2.352 11.622a.75.75 0 0 1 1.026-.27L12 16.382l8.622-5.03a.75.75 0 0 1 .756 1.296l-9 5.25a.75.75 0 0 1-.756 0l-9-5.25a.75.75 0 0 1-.27-1.026" />
      <path d="M2.352 16.122a.75.75 0 0 1 1.026-.27L12 20.882l8.622-5.03a.75.75 0 0 1 .756 1.296l-9 5.25a.75.75 0 0 1-.756 0l-9-5.25a.75.75 0 0 1-.27-1.026" />
    </Icon>
  )
);
