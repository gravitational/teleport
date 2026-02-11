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

export const Sparkle = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-sparkle"
      {...otherProps}
      ref={ref}
    >
      <path d="M16.5.75a.75.75 0 0 1 .75.75V3h1.5a.75.75 0 0 1 0 1.5h-1.5V6a.75.75 0 0 1-1.5 0V4.5h-1.5a.75.75 0 0 1 0-1.5h1.5V1.5a.75.75 0 0 1 .75-.75" />
      <path
        fillRule="evenodd"
        d="M9.698 4.501a1.404 1.404 0 0 1 2.118.664l.002.005 1.87 5.144 5.147 1.871a1.404 1.404 0 0 1 0 2.633l-.005.002-5.147 1.872-1.87 5.142a1.404 1.404 0 0 1-2.632 0l-.002-.005-1.867-5.143-5.147-1.871a1.404 1.404 0 0 1 0-2.633l.005-.002 5.142-1.866 1.872-5.149c.1-.268.28-.5.514-.664m.802 1.434 1.794 4.936a1.4 1.4 0 0 0 .836.836l4.935 1.795-4.93 1.792-.002.001a1.4 1.4 0 0 0-.84.828l-1.796 4.941-1.79-4.933-.001-.002a1.4 1.4 0 0 0-.836-.836l-4.935-1.795 4.933-1.79.002-.001a1.4 1.4 0 0 0 .836-.836v-.001z"
        clipRule="evenodd"
      />
      <path d="M21.75 6.75a.75.75 0 0 0-1.5 0v.75h-.75a.75.75 0 0 0 0 1.5h.75v.75a.75.75 0 0 0 1.5 0V9h.75a.75.75 0 0 0 0-1.5h-.75z" />
    </Icon>
  )
);
