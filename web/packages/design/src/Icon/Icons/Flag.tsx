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

export const Flag = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-flag"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M3.259 3.933c1.653-1.431 3.268-1.828 4.845-1.64 1.511.181 2.943.899 4.229 1.535 1.339.663 2.533 1.244 3.74 1.389 1.142.136 2.34-.117 3.686-1.284A.75.75 0 0 1 21 4.5v11.25a.75.75 0 0 1-.259.567c-1.653 1.432-3.268 1.827-4.845 1.639-1.511-.18-2.943-.898-4.229-1.534-1.339-.663-2.533-1.245-3.74-1.389-1.068-.127-2.185.086-3.427 1.07v4.147a.75.75 0 0 1-1.5 0V4.5l.002-.043v-.018a.8.8 0 0 1 .041-.183q.012-.038.027-.072l.028-.05a.8.8 0 0 1 .16-.2m4.668-.15c-1.068-.127-2.185.086-3.427 1.07v9.439c1.223-.703 2.424-.89 3.604-.748 1.511.18 2.943.898 4.229 1.534 1.339.663 2.533 1.244 3.74 1.389 1.068.127 2.185-.087 3.427-1.07v-9.44c-1.223.704-2.424.89-3.604.749-1.511-.18-2.943-.898-4.229-1.534-1.339-.663-2.533-1.245-3.74-1.389"
        clipRule="evenodd"
      />
    </Icon>
  )
);
