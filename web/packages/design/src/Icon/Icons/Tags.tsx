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

export const Tags = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-tags"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M3.75 3C3.33579 3 3 3.33579 3 3.75V12.44C3.00018 12.8373 3.15835 13.2187 3.43912 13.4998L12.7533 22.8111C13.0346 23.0922 13.4159 23.2501 13.8136 23.2501C14.2112 23.2501 14.5929 23.0919 14.8742 22.8109L22.8111 14.8711C23.0922 14.5898 23.2501 14.2084 23.2501 13.8108C23.2501 13.4131 23.092 13.0316 22.811 12.7503L13.4994 3.43875C13.2183 3.15798 12.837 3.00018 12.4397 3H3.75ZM4.5 12.4393L4.5 4.5H12.4393L21.7501 13.8108L13.8136 21.7501L4.5 12.4393ZM8.8125 7.875C8.8125 8.39277 8.39277 8.8125 7.875 8.8125C7.35723 8.8125 6.9375 8.39277 6.9375 7.875C6.9375 7.35723 7.35723 6.9375 7.875 6.9375C8.39277 6.9375 8.8125 7.35723 8.8125 7.875Z"
      />
    </Icon>
  )
);
