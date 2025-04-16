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

export const Application = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-application"
      {...otherProps}
      ref={ref}
    >
      <path d="M6.375 8.8125C6.89277 8.8125 7.3125 8.39277 7.3125 7.875C7.3125 7.35723 6.89277 6.9375 6.375 6.9375C5.85723 6.9375 5.4375 7.35723 5.4375 7.875C5.4375 8.39277 5.85723 8.8125 6.375 8.8125Z" />
      <path d="M11.0625 7.875C11.0625 8.39277 10.6428 8.8125 10.125 8.8125C9.60723 8.8125 9.1875 8.39277 9.1875 7.875C9.1875 7.35723 9.60723 6.9375 10.125 6.9375C10.6428 6.9375 11.0625 7.35723 11.0625 7.875Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M2.25 5.25C2.25 4.42157 2.92157 3.75 3.75 3.75H20.25C21.0784 3.75 21.75 4.42157 21.75 5.25V18.75C21.75 19.5784 21.0784 20.25 20.25 20.25H3.75C2.92157 20.25 2.25 19.5784 2.25 18.75V5.25ZM20.25 5.25H3.75V18.75H20.25V5.25Z"
      />
    </Icon>
  )
);
