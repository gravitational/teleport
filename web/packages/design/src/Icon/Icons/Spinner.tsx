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

export const Spinner = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-spinner"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M12 2a.77.77 0 0 1 .77.77v3.076a.77.77 0 1 1-1.54 0V2.77A.77.77 0 0 1 12 2m7.07 2.93c.301.3.301.787 0 1.087l-2.175 2.176a.77.77 0 0 1-1.088-1.087l2.176-2.176c.3-.3.787-.3 1.088 0M17.385 12a.77.77 0 0 1 .769-.77h3.077a.77.77 0 0 1 0 1.54h-3.077a.77.77 0 0 1-.77-.77m-1.577 3.807c.3-.3.787-.3 1.088 0l2.176 2.176a.77.77 0 0 1-1.088 1.088l-2.176-2.176a.77.77 0 0 1 0-1.088M12 17.385a.77.77 0 0 1 .77.769v3.077a.77.77 0 0 1-1.54 0v-3.077a.77.77 0 0 1 .77-.77m-3.807-1.577c.3.3.3.787 0 1.088L6.017 19.07a.77.77 0 1 1-1.088-1.088l2.176-2.176c.3-.3.788-.3 1.088 0M2 12a.77.77 0 0 1 .77-.77h3.076a.77.77 0 1 1 0 1.54H2.77A.77.77 0 0 1 2 12m2.93-7.07c.3-.3.787-.3 1.087 0l2.176 2.176a.77.77 0 0 1-1.088 1.087L4.93 6.017a.77.77 0 0 1 0-1.087"
        clipRule="evenodd"
      />
    </Icon>
  )
);
