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

export const Monitor = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-monitor"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M4 3.803c-.262.051-.794.329-.989.518-.268.259-.441.487-.551.728-.225.491-.219.31-.219 6.211 0 5.983-.01 5.727.247 6.251.178.366.635.823 1.001 1.001.533.261.044.247 8.511.247s7.978.014 8.511-.247c.366-.178.823-.635 1.001-1.001.257-.524.247-.268.247-6.251 0-5.901.006-5.72-.219-6.211-.271-.593-.954-1.135-1.575-1.249-.271-.05-15.706-.047-15.965.003m15.82 1.523c.159.088.325.267.377.407.03.077.043 1.8.043 5.532 0 4.917-.006 5.431-.065 5.547a.7.7 0 0 1-.344.353c-.145.074-.351.075-7.883.065l-7.734-.01-.147-.112a.9.9 0 0 1-.227-.269c-.079-.153-.08-.243-.08-5.579 0-4.464.01-5.445.054-5.552.067-.16.31-.379.457-.411.06-.013.127-.03.149-.038.022-.007 3.469-.01 7.66-.006 7.018.006 7.629.012 7.74.073M8.805 20.28c-.391.092-.633.519-.524.924.059.218.288.453.5.511.223.062 6.215.062 6.438 0a.84.84 0 0 0 .306-.191.739.739 0 0 0-.348-1.246c-.193-.044-6.183-.043-6.372.002"
      />
    </Icon>
  )
);
