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

export const ShieldCheck = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-shieldcheck"
      {...otherProps}
      ref={ref}
    >
      <path d="M16.2803 10.2803C16.5732 9.98744 16.5732 9.51256 16.2803 9.21967C15.9874 8.92678 15.5126 8.92678 15.2197 9.21967L10.5 13.9393L8.78033 12.2197C8.48744 11.9268 8.01256 11.9268 7.71967 12.2197C7.42678 12.5126 7.42678 12.9874 7.71967 13.2803L9.96967 15.5303C10.2626 15.8232 10.7374 15.8232 11.0303 15.5303L16.2803 10.2803Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M4.5 3.75C4.10217 3.75 3.72064 3.90804 3.43934 4.18934C3.15804 4.47064 3 4.85217 3 5.25V10.7616C3 19.1829 10.1444 21.9611 11.5298 22.4204C11.8348 22.5244 12.1656 22.5244 12.4706 22.4203C13.8537 21.9599 21 19.1792 21 10.7597V5.25C21 4.85218 20.842 4.47065 20.5607 4.18934C20.2794 3.90804 19.8978 3.75 19.5 3.75H4.5ZM4.5 5.25L19.5 5.25V10.7597C19.5 18.1058 13.3064 20.5606 12 20.996C10.6965 20.5635 4.5 18.112 4.5 10.7616L4.5 5.25Z"
      />
    </Icon>
  )
);
