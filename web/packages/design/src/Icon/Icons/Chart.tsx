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

import React from 'react';

import { Icon, IconProps } from '../Icon';

/*

THIS FILE IS GENERATED. DO NOT EDIT.

*/

export function Chart({ size = 24, color, ...otherProps }: IconProps) {
  return (
    <Icon size={size} color={color} className="icon icon-chart" {...otherProps}>
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M14.25 3C13.8358 3 13.5 3.33579 13.5 3.75V7.5H9C8.58579 7.5 8.25 7.83579 8.25 8.25V12H4.5C4.08579 12 3.75 12.3358 3.75 12.75V18.75H3C2.58579 18.75 2.25 19.0858 2.25 19.5C2.25 19.9142 2.58579 20.25 3 20.25H21C21.4142 20.25 21.75 19.9142 21.75 19.5C21.75 19.0858 21.4142 18.75 21 18.75H20.25V3.75C20.25 3.33579 19.9142 3 19.5 3H14.25ZM18.75 18.75V4.5H15V18.75H18.75ZM13.5 18.75V9H9.75V18.75H13.5ZM5.25 13.5H8.25V18.75H5.25V13.5Z"
      />
    </Icon>
  );
}
