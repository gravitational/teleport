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
      <path d="M18.5625 10.875C18.5625 11.3928 18.1428 11.8125 17.625 11.8125C17.1072 11.8125 16.6875 11.3928 16.6875 10.875C16.6875 10.3572 17.1072 9.9375 17.625 9.9375C18.1428 9.9375 18.5625 10.3572 18.5625 10.875Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M6 3C5.58579 3 5.25 3.33579 5.25 3.75V6.75H3.87469C2.62094 6.75 1.5 7.70164 1.5 9V16.5C1.5 16.9142 1.83579 17.25 2.25 17.25H5.25V20.25C5.25 20.6642 5.58579 21 6 21H18C18.4142 21 18.75 20.6642 18.75 20.25V17.25H21.75C22.1642 17.25 22.5 16.9142 22.5 16.5V9C22.5 7.70164 21.3791 6.75 20.1253 6.75H18.75V3.75C18.75 3.33579 18.4142 3 18 3H6ZM3 9C3 8.64086 3.33406 8.25 3.87469 8.25H20.1253C20.6659 8.25 21 8.64086 21 9V15.75H18.75V14.25C18.75 13.8358 18.4142 13.5 18 13.5H6C5.58579 13.5 5.25 13.8358 5.25 14.25V15.75H3V9ZM6.75 6.75H17.25V4.5H6.75V6.75ZM6.75 15H17.25V19.5H6.75V15Z"
      />
    </Icon>
  )
);
