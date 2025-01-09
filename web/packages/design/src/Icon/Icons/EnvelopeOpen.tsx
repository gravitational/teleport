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

export const EnvelopeOpen = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-envelopeopen"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M12.416 2.37596C12.1641 2.20801 11.8359 2.20801 11.584 2.37596L2.59504 8.36858C2.54568 8.40022 2.49953 8.43803 2.4578 8.48179C2.41124 8.53051 2.37167 8.58508 2.33994 8.64388C2.27938 8.75578 2.25003 8.87799 2.25 8.99926C2.25 8.99951 2.25 8.99902 2.25 8.99926L2.25 18.75C2.25 19.1478 2.40804 19.5294 2.68934 19.8107C2.97065 20.092 3.35218 20.25 3.75 20.25H20.25C20.6478 20.25 21.0294 20.092 21.3107 19.8107C21.592 19.5294 21.75 19.1478 21.75 18.75V9.01273C21.7527 8.85774 21.7076 8.70055 21.6107 8.56465C21.5528 8.48342 21.4817 8.4172 21.4023 8.36684L12.416 2.37596ZM19.6791 9.02079L12 3.90139L4.32101 9.02071L10.6041 13.5001H13.3969L19.6791 9.02079ZM3.75 10.4558V18.75H20.25V10.456L14.0723 14.8607C13.9452 14.9514 13.793 15.0001 13.6369 15.0001H10.3641C10.208 15.0001 10.0558 14.9514 9.92874 14.8608L3.75 10.4558Z"
      />
    </Icon>
  )
);
