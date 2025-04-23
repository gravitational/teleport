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

export const Share = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-share"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M13.9629 2.30711C14.2431 2.19103 14.5657 2.25519 14.7802 2.46969L22.2802 9.96969C22.5731 10.2626 22.5731 10.7375 22.2802 11.0304L14.7802 18.5303C14.5657 18.7448 14.2431 18.809 13.9629 18.6929C13.6826 18.5768 13.4999 18.3034 13.4999 18V14.271C8.44754 14.5539 4.86823 17.6301 3.44933 19.1402C3.2893 19.3139 3.07865 19.4328 2.84717 19.4801C2.61314 19.5278 2.36998 19.4999 2.15288 19.4003C1.93579 19.3007 1.75603 19.1345 1.63965 18.926C1.52372 18.7182 1.47666 18.479 1.50519 18.2429C1.88497 14.9597 3.7728 12.0912 6.17252 10.0557C8.34631 8.21181 11.0114 6.99094 13.4999 6.78197V3.00002C13.4999 2.69668 13.6826 2.4232 13.9629 2.30711ZM14.9999 4.81068V7.50002C14.9999 7.91424 14.6641 8.25002 14.2499 8.25002C11.9934 8.25002 9.33534 9.33983 7.14282 11.1996C5.2549 12.801 3.77667 14.9185 3.1992 17.2813C5.20258 15.4393 9.05613 12.75 14.2499 12.75C14.6641 12.75 14.9999 13.0858 14.9999 13.5V16.1894L20.6892 10.5L14.9999 4.81068ZM2.24996 18.3319H2.24996H2.24996Z"
      />
    </Icon>
  )
);
