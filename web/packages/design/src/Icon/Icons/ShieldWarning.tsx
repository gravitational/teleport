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

export const ShieldWarning = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-shieldwarning"
      {...otherProps}
      ref={ref}
    >
      <path d="M12 8.25a.75.75 0 0 1 .75.75v3.75a.75.75 0 0 1-1.5 0V9a.75.75 0 0 1 .75-.75m.938 7.875a.937.937 0 1 1-1.875 0 .937.937 0 0 1 1.874 0" />
      <path
        fillRule="evenodd"
        d="M3.44 4.19a1.5 1.5 0 0 1 1.06-.44h15a1.5 1.5 0 0 1 1.5 1.5v5.51c0 8.42-7.146 11.2-8.53 11.66a1.46 1.46 0 0 1-.94 0C10.144 21.961 3 19.183 3 10.762V5.25c0-.398.158-.78.44-1.06M19.5 5.25h-15v5.512c0 7.35 6.197 9.802 7.5 10.234 1.306-.435 7.5-2.89 7.5-10.236z"
        clipRule="evenodd"
      />
    </Icon>
  )
);
