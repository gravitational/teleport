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

export const Logout = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-logout"
      {...otherProps}
      ref={ref}
    >
      <path d="M3.43934 3.43934C3.72064 3.15804 4.10217 3 4.5 3H9.75C10.1642 3 10.5 3.33579 10.5 3.75C10.5 4.16421 10.1642 4.5 9.75 4.5L4.5 4.5L4.5 19.5H9.75C10.1642 19.5 10.5 19.8358 10.5 20.25C10.5 20.6642 10.1642 21 9.75 21H4.5C4.10218 21 3.72065 20.842 3.43934 20.5607C3.15804 20.2794 3 19.8978 3 19.5V4.5C3 4.10217 3.15804 3.72064 3.43934 3.43934Z" />
      <path d="M9 12C9 11.5858 9.33579 11.25 9.75 11.25H18.4393L15.9697 8.78033C15.6768 8.48744 15.6768 8.01256 15.9697 7.71967C16.2626 7.42678 16.7374 7.42678 17.0303 7.71967L20.7803 11.4697C20.8522 11.5416 20.9065 11.6245 20.9431 11.7129C20.9751 11.7901 20.9946 11.8738 20.999 11.9614C20.9997 11.9742 21 11.9871 21 12C21 12.1017 20.9798 12.1987 20.9431 12.2871C20.9065 12.3755 20.8522 12.4584 20.7803 12.5303L17.0303 16.2803C16.7374 16.5732 16.2626 16.5732 15.9697 16.2803C15.6768 15.9874 15.6768 15.5126 15.9697 15.2197L18.4393 12.75H9.75C9.33579 12.75 9 12.4142 9 12Z" />
    </Icon>
  )
);
