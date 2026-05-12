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

export const Beams = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-beams"
      {...otherProps}
      ref={ref}
    >
      <path d="M18.75 7.777c0-.557-.142-.9-.331-1.13-.198-.24-.51-.437-.984-.584-.987-.305-2.37-.313-3.994-.313H4.75v5.397h8.847c1.556 0 3.294.068 4.654.61.696.279 1.334.697 1.795 1.327.464.636.704 1.428.704 2.374s-.24 1.738-.705 2.373c-.461.629-1.1 1.043-1.796 1.318-1.36.537-3.098.601-4.652.601H4a.75.75 0 0 1-.75-.75v-3.5a.75.75 0 0 1 1.5 0v2.75h8.847c1.577 0 3.041-.077 4.1-.495.513-.203.888-.47 1.138-.81.246-.336.415-.803.415-1.487s-.17-1.152-.416-1.49c-.25-.342-.626-.612-1.139-.817-1.058-.422-2.523-.505-4.098-.505H4a.75.75 0 0 1-.75-.75V5A.75.75 0 0 1 4 4.25h9.441c1.54 0 3.186-.007 4.438.38.645.201 1.254.525 1.698 1.065.453.551.673 1.251.673 2.082q0 1-.505 1.734a.75.75 0 0 1-1.236-.849c.145-.21.241-.484.241-.885" />
    </Icon>
  )
);
