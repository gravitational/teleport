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
      <path d="M12 8.25C12.4142 8.25 12.75 8.58579 12.75 9V12.75C12.75 13.1642 12.4142 13.5 12 13.5C11.5858 13.5 11.25 13.1642 11.25 12.75V9C11.25 8.58579 11.5858 8.25 12 8.25Z" />
      <path d="M12.9375 16.125C12.9375 16.6428 12.5178 17.0625 12 17.0625C11.4822 17.0625 11.0625 16.6428 11.0625 16.125C11.0625 15.6072 11.4822 15.1875 12 15.1875C12.5178 15.1875 12.9375 15.6072 12.9375 16.125Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M3.43934 4.18934C3.72064 3.90804 4.10217 3.75 4.5 3.75H19.5C19.8978 3.75 20.2794 3.90804 20.5607 4.18934C20.842 4.47065 21 4.85218 21 5.25V10.7597C21 19.1792 13.8537 21.9599 12.4706 22.4203C12.1656 22.5244 11.8348 22.5244 11.5298 22.4204C10.1444 21.9611 3 19.1829 3 10.7616V5.25C3 4.85217 3.15804 4.47064 3.43934 4.18934ZM19.5 5.25L4.5 5.25L4.5 10.7616C4.5 18.112 10.6965 20.5635 12 20.996C13.3064 20.5606 19.5 18.1058 19.5 10.7597V5.25Z"
      />
    </Icon>
  )
);
