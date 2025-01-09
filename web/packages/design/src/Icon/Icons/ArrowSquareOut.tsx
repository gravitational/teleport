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

export const ArrowSquareOut = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-arrowsquareout"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M13.5 3.74988C13.5001 3.33567 13.8359 2.99994 14.2501 3L20.2125 3.00093C20.4167 2.99077 20.6243 3.06369 20.7803 3.21967C20.9363 3.37565 21.0092 3.58324 20.9991 3.78747L21 9.74988C21.0001 10.1641 20.6643 10.4999 20.2501 10.5C19.8359 10.5001 19.5001 10.1643 19.5 9.75012L19.4993 5.56132L13.2803 11.7803C12.9874 12.0732 12.5126 12.0732 12.2197 11.7803C11.9268 11.4874 11.9268 11.0126 12.2197 10.7197L18.4387 4.50065L14.2499 4.5C13.8357 4.49994 13.4999 4.1641 13.5 3.74988ZM4.5 6C4.10217 6 3.72064 6.15804 3.43934 6.43934C3.15804 6.72064 3 7.10217 3 7.5V19.5C3 19.8978 3.15804 20.2794 3.43934 20.5607C3.72065 20.842 4.10218 21 4.5 21H16.5C16.8978 21 17.2794 20.842 17.5607 20.5607C17.842 20.2794 18 19.8978 18 19.5V12.75C18 12.3358 17.6642 12 17.25 12C16.8358 12 16.5 12.3358 16.5 12.75V19.5H4.5L4.5 7.5H11.25C11.6642 7.5 12 7.16421 12 6.75C12 6.33579 11.6642 6 11.25 6H4.5Z"
      />
    </Icon>
  )
);
