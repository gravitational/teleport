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

export const Pencil = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-pencil"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M16.0608 2.25C15.6631 2.25 15.2817 2.4079 15.0005 2.68899L12.2278 5.46169C12.2251 5.46432 12.2224 5.46698 12.2197 5.46967C12.217 5.47235 12.2143 5.47505 12.2117 5.47777L3.43875 14.2507C3.15798 14.5318 3.00018 14.9131 3 15.3104V19.5001C3 19.8979 3.15804 20.2795 3.43934 20.5608C3.72065 20.8421 4.10218 21.0001 4.5 21.0001H8.69003C9.08733 20.9999 9.46868 20.8418 9.74977 20.561L18.4964 11.8123C18.508 11.802 18.5193 11.7914 18.5303 11.7803C18.5414 11.7692 18.5521 11.7579 18.5624 11.7463L21.3111 8.99684C21.5922 8.71556 21.7501 8.33418 21.7501 7.93653C21.7501 7.53887 21.5919 7.15714 21.3108 6.87586L17.1211 2.68899C16.8398 2.4079 16.4584 2.25 16.0608 2.25ZM17.9989 10.1883L20.2501 7.93653L16.0608 3.75L13.8107 6.00006L17.9989 10.1883ZM12.7501 7.06072L4.5 15.3108V19.5001L8.68934 19.5001L16.9384 11.2491L12.7501 7.06072Z"
      />
    </Icon>
  )
);
