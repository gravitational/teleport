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

export const Floppy = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-floppy"
      {...otherProps}
      ref={ref}
    >
      <path
        d="M4.174 3.042c-.418.082-.844.433-1.042.858l-.112.24v15.72l.112.24c.147.315.455.624.764.767l.244.113h15.72l.244-.113c.309-.143.617-.452.764-.767l.112-.24.012-5.7c.013-5.975.014-5.948-.161-6.3-.097-.195-4.271-4.394-4.581-4.608-.377-.261-.125-.251-6.31-.247-3.08.002-5.675.019-5.766.037M17.47 6.53l2.01 2.01v10.981l-1.11-.01-1.11-.011-.02-2.82-.02-2.82-.108-.229a1.7 1.7 0 0 0-.743-.743l-.229-.108H7.86l-.229.108a1.7 1.7 0 0 0-.743.743l-.108.229-.02 2.82-.02 2.82-1.11.011-1.11.01V4.52h10.94l2.01 2.01m-8.81-.47c-.296.105-.451.409-.406.794.049.409.265.605.717.645.159.015 1.468.021 2.909.014L14.5 7.5l.164-.094c.22-.126.304-.282.325-.602.021-.324-.09-.576-.309-.703-.137-.079-.21-.081-3-.09-2.394-.007-2.886.001-3.02.049m7.09 10.81-.01 2.63H8.26l-.01-2.63-.011-2.63h7.522l-.011 2.63"
        fillRule="evenodd"
      />
    </Icon>
  )
);
