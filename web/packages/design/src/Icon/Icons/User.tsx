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

export const User = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-user"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M12 2.25C8.27208 2.25 5.25 5.27208 5.25 9C5.25 11.4639 6.57018 13.6195 8.54176 14.798C5.90407 15.664 3.72635 17.4974 2.35075 19.8743C2.14327 20.2328 2.2657 20.6917 2.62421 20.8991C2.98272 21.1066 3.44154 20.9842 3.64901 20.6257C5.34045 17.703 8.40021 15.75 11.9999 15.75C15.5996 15.75 18.6593 17.703 20.3507 20.6257C20.5582 20.9842 21.017 21.1066 21.3755 20.8991C21.7341 20.6917 21.8565 20.2328 21.649 19.8743C20.2734 17.4974 18.0958 15.664 15.4582 14.7981C17.4298 13.6196 18.75 11.464 18.75 9C18.75 5.27208 15.7279 2.25 12 2.25ZM6.75 9C6.75 6.1005 9.1005 3.75 12 3.75C14.8995 3.75 17.25 6.1005 17.25 9C17.25 11.8995 14.8995 14.25 12 14.25C9.1005 14.25 6.75 11.8995 6.75 9Z"
      />
    </Icon>
  )
);
