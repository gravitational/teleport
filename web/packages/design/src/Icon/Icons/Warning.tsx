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

export const Warning = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-warning"
      {...otherProps}
      ref={ref}
    >
      <path d="M12 9C12.4142 9 12.75 9.33579 12.75 9.75V13.5C12.75 13.9142 12.4142 14.25 12 14.25C11.5858 14.25 11.25 13.9142 11.25 13.5V9.75C11.25 9.33579 11.5858 9 12 9Z" />
      <path d="M12 17.8125C12.5178 17.8125 12.9375 17.3928 12.9375 16.875C12.9375 16.3572 12.5178 15.9375 12 15.9375C11.4822 15.9375 11.0625 16.3572 11.0625 16.875C11.0625 17.3928 11.4822 17.8125 12 17.8125Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M10.09 3.34287L1.88101 17.7086C1.04292 19.1753 2.10193 21.0001 3.79114 21.0001H20.2092C21.8984 21.0001 22.9574 19.1753 22.1193 17.7086L13.9103 3.34287C13.0657 1.86488 10.9346 1.86488 10.09 3.34287ZM3.18337 18.4528L11.3924 4.08708C11.6611 3.61681 12.3392 3.61681 12.6079 4.08708L20.8169 18.4528C21.0836 18.9195 20.7466 19.5001 20.2092 19.5001H3.79114C3.25367 19.5001 2.91671 18.9195 3.18337 18.4528Z"
      />
    </Icon>
  )
);
