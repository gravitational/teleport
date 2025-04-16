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

export const Edit = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-edit"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M17.4697 2.46967C17.7626 2.17678 18.2374 2.17678 18.5303 2.46967L21.5303 5.46967C21.8232 5.76256 21.8232 6.23744 21.5303 6.53033L12.5303 15.5303C12.3897 15.671 12.1989 15.75 12 15.75H9C8.58579 15.75 8.25 15.4142 8.25 15V12C8.25 11.8011 8.32902 11.6103 8.46967 11.4697L17.4697 2.46967ZM19.9393 6L18.75 7.18934L16.8107 5.25L18 4.06066L19.9393 6ZM9.75 12.3107L15.75 6.31066L17.6893 8.25L11.6893 14.25H9.75V12.3107Z"
      />
      <path d="M4.5 3C4.10217 3 3.72064 3.15804 3.43934 3.43934C3.15804 3.72064 3 4.10217 3 4.5V19.5C3 19.8978 3.15804 20.2794 3.43934 20.5607C3.72065 20.842 4.10218 21 4.5 21H19.5C19.8978 21 20.2794 20.842 20.5607 20.5607C20.842 20.2794 21 19.8978 21 19.5V11.25C21 10.8358 20.6642 10.5 20.25 10.5C19.8358 10.5 19.5 10.8358 19.5 11.25V19.5H4.5L4.5 4.5L12.75 4.5C13.1642 4.5 13.5 4.16421 13.5 3.75C13.5 3.33579 13.1642 3 12.75 3H4.5Z" />
    </Icon>
  )
);
