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

export const FolderPlus = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-folderplus"
      {...otherProps}
      ref={ref}
    >
      <path d="M9 13.5C9 13.0858 9.33579 12.75 9.75 12.75H11.25V11.25C11.25 10.8358 11.5858 10.5 12 10.5C12.4142 10.5 12.75 10.8358 12.75 11.25V12.75H14.25C14.6642 12.75 15 13.0858 15 13.5C15 13.9142 14.6642 14.25 14.25 14.25H12.75V15.75C12.75 16.1642 12.4142 16.5 12 16.5C11.5858 16.5 11.25 16.1642 11.25 15.75V14.25H9.75C9.33579 14.25 9 13.9142 9 13.5Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M2.68934 4.93934C2.97064 4.65804 3.35217 4.5 3.75 4.5H8.74969C9.07421 4.5 9.38999 4.60525 9.64962 4.79995L12.2502 6.74995L20.25 6.75C20.6478 6.75 21.0294 6.90804 21.3107 7.18934C21.592 7.47065 21.75 7.85218 21.75 8.25V18.8334C21.75 19.2091 21.6008 19.5694 21.3351 19.8351C21.0694 20.1008 20.7091 20.25 20.3334 20.25H3.75C3.35218 20.25 2.97065 20.092 2.68934 19.8107C2.40804 19.5294 2.25 19.1478 2.25 18.75V6C2.25 5.60217 2.40804 5.22064 2.68934 4.93934ZM8.74969 6L3.75 6L3.75 18.75H20.25V8.25H12.2503C11.9258 8.25 11.61 8.14475 11.3504 7.95005L8.74969 6Z"
      />
    </Icon>
  )
);
