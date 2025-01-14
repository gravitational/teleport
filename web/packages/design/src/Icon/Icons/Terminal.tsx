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

export const Terminal = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-terminal"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M3.75 3.75C2.92157 3.75 2.25 4.42157 2.25 5.25V18.75C2.25 19.5784 2.92157 20.25 3.75 20.25H20.25C21.0784 20.25 21.75 19.5784 21.75 18.75V5.25C21.75 4.42157 21.0784 3.75 20.25 3.75H3.75ZM3.75 5.25H20.25V18.75H3.75V5.25ZM6.91438 8.53151C7.17313 8.20806 7.6451 8.15562 7.96855 8.41438L11.7185 11.4144C11.8965 11.5567 12 11.7722 12 12C12 12.2279 11.8965 12.4433 11.7185 12.5857L7.96855 15.5857C7.6451 15.8444 7.17313 15.792 6.91438 15.4685C6.65562 15.1451 6.70806 14.6731 7.03151 14.4144L10.0494 12L7.03151 9.58568C6.70806 9.32692 6.65562 8.85495 6.91438 8.53151ZM12 15C12 14.5858 12.3358 14.25 12.75 14.25H16.5C16.9142 14.25 17.25 14.5858 17.25 15C17.25 15.4142 16.9142 15.75 16.5 15.75H12.75C12.3358 15.75 12 15.4142 12 15Z"
      />
    </Icon>
  )
);
