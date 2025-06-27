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

export const Wrench = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-wrench"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M15.874 3.822a5.25 5.25 0 0 0-5.468 7.716.75.75 0 0 1-.166.93l-6.07 5.237a1.504 1.504 0 0 0 2.126 2.126l5.235-6.07a.75.75 0 0 1 .931-.167 5.25 5.25 0 0 0 7.716-5.467l-3.17 2.925a.75.75 0 0 1-.666.182l-2.469-.531a.75.75 0 0 1-.576-.576l-.53-2.47a.75.75 0 0 1 .182-.665zm-2.417-1.394a6.75 6.75 0 0 1 4.074.313.75.75 0 0 1 .27 1.204l-3.486 3.778.347 1.615 1.615.347L20.055 6.2a.75.75 0 0 1 1.204.27 6.75 6.75 0 0 1-8.97 8.71l-4.877 5.655-.038.04a3.004 3.004 0 1 1-4.208-4.285l5.655-4.877a6.75 6.75 0 0 1 4.636-9.284"
        clipRule="evenodd"
      />
    </Icon>
  )
);
