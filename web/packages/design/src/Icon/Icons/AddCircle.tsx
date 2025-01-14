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

export const AddCircle = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-addcircle"
      {...otherProps}
      ref={ref}
    >
      <path d="M6.46154 12C6.46154 11.4902 6.87481 11.0769 7.38462 11.0769H11.0769V7.38462C11.0769 6.87481 11.4902 6.46154 12 6.46154C12.5098 6.46154 12.9231 6.87481 12.9231 7.38462V11.0769H16.6154C17.1252 11.0769 17.5385 11.4902 17.5385 12C17.5385 12.5098 17.1252 12.9231 16.6154 12.9231H12.9231V16.6154C12.9231 17.1252 12.5098 17.5385 12 17.5385C11.4902 17.5385 11.0769 17.1252 11.0769 16.6154V12.9231H7.38462C6.87481 12.9231 6.46154 12.5098 6.46154 12Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M0 12C0 5.37258 5.37258 0 12 0C18.6274 0 24 5.37258 24 12C24 18.6274 18.6274 24 12 24C5.37258 24 0 18.6274 0 12ZM12 1.84615C6.39219 1.84615 1.84615 6.39219 1.84615 12C1.84615 17.6078 6.39219 22.1538 12 22.1538C17.6078 22.1538 22.1538 17.6078 22.1538 12C22.1538 6.39219 17.6078 1.84615 12 1.84615Z"
      />
    </Icon>
  )
);
