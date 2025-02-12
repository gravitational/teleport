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

export const Ruler = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-ruler"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M1.93958 15C1.3538 15.5858 1.3538 16.5356 1.93958 17.1214L6.87895 22.0607C7.46473 22.6465 8.41448 22.6465 9.00027 22.0607L22.061 9.00002C22.6468 8.41424 22.6468 7.46449 22.061 6.8787L17.1216 1.93934C16.5358 1.35355 15.5861 1.35355 15.0003 1.93934L11.4776 5.46205L11.4697 5.46983L11.4619 5.47772L8.47757 8.46205L8.46969 8.46983L8.46191 8.47771L5.47756 11.4621C5.47492 11.4646 5.4723 11.4672 5.46969 11.4698C5.46708 11.4724 5.46449 11.4751 5.46193 11.4777L1.93958 15ZM6.00007 13.0609L3.00024 16.0607L7.93961 21.0001L21.0003 7.93936L16.0609 3L13.0607 6.00021L15.5303 8.46983C15.8232 8.76272 15.8232 9.23759 15.5303 9.53049C15.2375 9.82338 14.7626 9.82338 14.4697 9.53049L12.0001 7.06087L10.0607 9.00021L12.5303 11.4698C12.8232 11.7627 12.8232 12.2376 12.5303 12.5305C12.2375 12.8234 11.7626 12.8234 11.4697 12.5305L9.00007 10.0609L7.06073 12.0002L9.53035 14.4698C9.82324 14.7627 9.82324 15.2376 9.53035 15.5305C9.23745 15.8234 8.76258 15.8234 8.46969 15.5305L6.00007 13.0609Z"
      />
    </Icon>
  )
);
