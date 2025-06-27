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

export const Layout = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-layout"
      {...otherProps}
      ref={ref}
    >
      <path d="M3.34 3.814a1.46 1.46 0 0 0-.666.413c-.226.235-.333.44-.39.752-.063.338-.062 13.774.001 14.069.113.535.516.979 1.029 1.135.279.084 17.093.084 17.372 0 .525-.16.929-.615 1.03-1.162.062-.331.062-13.711 0-14.042a1.37 1.37 0 0 0-.423-.784 1.5 1.5 0 0 0-.673-.393c-.239-.062-17.065-.05-17.28.012m16.9 3.306V9H3.76V5.24h16.48zM9 14.64v4.12H3.76v-8.24H9zm11.24 0v4.12h-9.72v-8.24h9.72z" />
    </Icon>
  )
);
