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

export const Vault = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-vault"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M2.25 5.25C2.25 4.42157 2.92157 3.75 3.75 3.75H20.25C21.0784 3.75 21.75 4.42157 21.75 5.25V18C21.75 18.8284 21.0784 19.5 20.25 19.5H18.75V21C18.75 21.4142 18.4142 21.75 18 21.75C17.5858 21.75 17.25 21.4142 17.25 21V19.5H6.75V21C6.75 21.4142 6.41421 21.75 6 21.75C5.58579 21.75 5.25 21.4142 5.25 21V19.5H3.75C2.92157 19.5 2.25 18.8284 2.25 18V5.25ZM20.25 5.25V11.25H18.6878C18.3307 9.12172 16.4797 7.5 14.25 7.5C11.7647 7.5 9.75 9.51472 9.75 12C9.75 14.4853 11.7647 16.5 14.25 16.5C16.4797 16.5 18.3307 14.8783 18.6878 12.75H20.25V18H3.75V5.25H20.25ZM17.1555 11.25H15.3272C15.0901 10.91 14.696 10.6875 14.25 10.6875C13.5251 10.6875 12.9375 11.2751 12.9375 12C12.9375 12.7249 13.5251 13.3125 14.25 13.3125C14.696 13.3125 15.0901 13.09 15.3272 12.75H17.1555C16.8225 14.0439 15.6479 15 14.25 15C12.5931 15 11.25 13.6569 11.25 12C11.25 10.3431 12.5931 9 14.25 9C15.6479 9 16.8225 9.95608 17.1555 11.25Z"
      />
    </Icon>
  )
);
