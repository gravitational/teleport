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
      <path
        fillRule="evenodd"
        d="M8.473 1.651a.63.63 0 0 1 .362.001l5.836 1.783c.227.07.383.256.427.473a1 1 0 0 1 .055.454l.008.088v4.63l1.145-.331.089-.019a.6.6 0 0 1 .27.022l5.653 1.74a.62.62 0 0 1 .438.607l.003 6.896c0 .276-.18.52-.445.599L8.916 22.602a.6.6 0 0 1-.144.023.6.6 0 0 1-.24-.025l-5.717-1.818a.625.625 0 0 1-.436-.596V14.25l-.004-1a.63.63 0 0 1 .45-.587l1.835-.533-1.844-.566a.63.63 0 0 1-.441-.597V3.95h.002a.62.62 0 0 1 .443-.602zm.889 14.393v5.121l12.147-3.635v-5.123zm5.799-5.661v.598c0 .277-.182.52-.447.6l-5.752 1.715a.63.63 0 0 1-.428.023l-1.697-.52-2.46.715 4.448 1.384 11.945-3.575-4.295-1.321zM9.358 6.649v5.226l4.553-1.358V5.289zm-4.98-2.465 4.34 1.351 4.323-1.291-4.39-1.342z"
        clipRule="evenodd"
      />
    </Icon>
  )
);
