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

export const Planet = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-planet"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        d="M3.073 13.151A9 9 0 0 1 17.5 4.876c1.166-.285 2.232-.42 3.116-.363.944.06 1.89.357 2.364 1.175.434.747.292 1.62-.047 2.394-.347.792-.963 1.643-1.759 2.502q-.12.13-.247.262.072.567.073 1.154a9 9 0 0 1-14.499 7.125q-.165.04-.325.076c-1.145.256-2.194.361-3.055.263-.842-.095-1.669-.41-2.101-1.157-.477-.822-.26-1.792.164-2.64.396-.794 1.052-1.65 1.889-2.516M4.5 12a7.5 7.5 0 0 1 14.814-1.667c-1.452 1.383-3.462 2.872-5.824 4.23-2.372 1.362-4.682 2.352-6.617 2.91A7.5 7.5 0 0 1 4.5 12m-1.013 2.93c-.43.515-.753.99-.961 1.407-.351.703-.299 1.063-.209 1.218l.001.001c.083.143.324.344.973.418.491.056 1.127.025 1.886-.104a9 9 0 0 1-1.69-2.94m4.893 3.64a7.5 7.5 0 0 0 11.118-6.38c-1.444 1.245-3.241 2.513-5.26 3.673-2.032 1.167-4.045 2.085-5.858 2.707m12.132-9.501c.499-.597.848-1.134 1.047-1.588.26-.594.205-.9.124-1.04-.09-.156-.38-.381-1.163-.431-.466-.03-1.036.009-1.697.121a9 9 0 0 1 1.689 2.938"
        clipRule="evenodd"
      />
    </Icon>
  )
);
