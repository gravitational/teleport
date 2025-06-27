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
      <path d="M9 13.5a.75.75 0 0 1 .75-.75h1.5v-1.5a.75.75 0 0 1 1.5 0v1.5h1.5a.75.75 0 0 1 0 1.5h-1.5v1.5a.75.75 0 0 1-1.5 0v-1.5h-1.5A.75.75 0 0 1 9 13.5" />
      <path
        fillRule="evenodd"
        d="M2.69 4.94a1.5 1.5 0 0 1 1.06-.44h5c.324 0 .64.105.9.3l2.6 1.95h8a1.5 1.5 0 0 1 1.5 1.5v10.583a1.417 1.417 0 0 1-1.417 1.417H3.75a1.5 1.5 0 0 1-1.5-1.5V6c0-.398.158-.78.44-1.06M8.75 6h-5v12.75h16.5V8.25h-8a1.5 1.5 0 0 1-.9-.3z"
        clipRule="evenodd"
      />
    </Icon>
  )
);
