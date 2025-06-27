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

export const Keyboard = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-keyboard"
      {...otherProps}
      ref={ref}
    >
      <path d="M5.25 11.25a.75.75 0 0 0 0 1.5h13.5a.75.75 0 0 0 0-1.5zM4.5 9a.75.75 0 0 1 .75-.75h13.5a.75.75 0 0 1 0 1.5H5.25A.75.75 0 0 1 4.5 9m.75 5.25a.75.75 0 0 0 0 1.5H6a.75.75 0 0 0 0-1.5zm3 .75a.75.75 0 0 1 .75-.75h6a.75.75 0 0 1 0 1.5H9a.75.75 0 0 1-.75-.75m9.75-.75a.75.75 0 0 0 0 1.5h.75a.75.75 0 0 0 0-1.5z" />
      <path
        fillRule="evenodd"
        d="M3.046 4.5c-.854 0-1.546.692-1.546 1.546v11.908c0 .854.692 1.546 1.546 1.546h17.908c.854 0 1.546-.692 1.546-1.546V6.046c0-.854-.692-1.546-1.546-1.546zM3 6.046C3 6.02 3.02 6 3.046 6h17.908c.025 0 .046.02.046.046v11.908c0 .025-.02.046-.046.046H3.046A.046.046 0 0 1 3 17.954z"
        clipRule="evenodd"
      />
    </Icon>
  )
);
