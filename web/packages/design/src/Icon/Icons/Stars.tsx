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

export const Stars = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-stars"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M10.624 2.411A1.51 1.51 0 0 1 12 1.508c.591 0 1.141.36 1.376.903l.004.008 2.178 5.19 5.568.482h.002c.598.05 1.121.464 1.305 1.035.184.57.003 1.21-.452 1.6l-4.222 3.682-.002.006-.002.007 1.267 5.483v.002a1.51 1.51 0 0 1-.576 1.55 1.51 1.51 0 0 1-1.653.078l-.005-.003-4.786-2.904h-.004l-4.79 2.907a1.51 1.51 0 0 1-1.654-.078 1.51 1.51 0 0 1-.576-1.55l1.267-5.485-.004-.012-4.222-3.683a1.51 1.51 0 0 1-.452-1.6c.184-.57.708-.985 1.305-1.034l5.57-.482zM12 3.008l2.174 5.181c.213.509.702.866 1.25.915h.003L21 9.586h.005l-4.237 3.696-.002.002a1.52 1.52 0 0 0-.474 1.471l1.268 5.489v.002l-4.785-2.904a1.51 1.51 0 0 0-1.552 0L6.44 20.246v-.002l1.268-5.488a1.52 1.52 0 0 0-.474-1.473l-4.231-3.69-.007-.007 5.578-.482h.002a1.51 1.51 0 0 0 1.25-.914m0 0L12 3.008z"
        clipRule="evenodd"
      />
    </Icon>
  )
);
