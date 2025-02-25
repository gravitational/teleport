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

export const Profile = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-profile"
      {...otherProps}
      ref={ref}
    >
      <path d="M13.5 10.5C13.5 10.0858 13.8358 9.75 14.25 9.75H18C18.4142 9.75 18.75 10.0858 18.75 10.5C18.75 10.9142 18.4142 11.25 18 11.25H14.25C13.8358 11.25 13.5 10.9142 13.5 10.5Z" />
      <path d="M13.5 13.5C13.5 13.0858 13.8358 12.75 14.25 12.75H18C18.4142 12.75 18.75 13.0858 18.75 13.5C18.75 13.9142 18.4142 14.25 18 14.25H14.25C13.8358 14.25 13.5 13.9142 13.5 13.5Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M6 11.25C6 9.59315 7.34315 8.25 9 8.25C10.6569 8.25 12 9.59315 12 11.25C12 12.0813 11.6619 12.8336 11.1157 13.3769C11.8951 13.8779 12.4913 14.6466 12.7264 15.5638C12.8293 15.965 12.5874 16.3737 12.1862 16.4765C11.7849 16.5794 11.3763 16.3375 11.2734 15.9362C11.0316 14.9928 10.0758 14.25 8.99994 14.25C7.92473 14.25 6.96904 14.9932 6.72629 15.9369C6.62309 16.338 6.21424 16.5795 5.81309 16.4764C5.41193 16.3732 5.17039 15.9643 5.27359 15.5631C5.50926 14.647 6.10527 13.8783 6.88457 13.3772C6.33822 12.8338 6 12.0814 6 11.25ZM9 9.75C8.17157 9.75 7.5 10.4216 7.5 11.25C7.5 12.0784 8.17157 12.75 9 12.75C9.82843 12.75 10.5 12.0784 10.5 11.25C10.5 10.4216 9.82843 9.75 9 9.75Z"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M3.75 3.75C2.92157 3.75 2.25 4.42157 2.25 5.25V18.75C2.25 19.5784 2.92157 20.25 3.75 20.25H20.25C21.0784 20.25 21.75 19.5784 21.75 18.75V5.25C21.75 4.42157 21.0784 3.75 20.25 3.75H3.75ZM3.75 5.25H20.25V18.75H3.75V5.25Z"
      />
    </Icon>
  )
);
