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

export const Home = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-home"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M12.0005 2.25C11.627 2.25 11.267 2.38932 10.9908 2.64071L3.48503 9.72783C3.33323 9.86734 3.21179 10.0367 3.12833 10.2252C3.04373 10.4164 3.00002 10.6232 3 10.8322V19.5H1.5C1.08579 19.5 0.75 19.8358 0.75 20.25C0.75 20.6642 1.08579 21 1.5 21H4.46753C4.47834 21.0002 4.48917 21.0004 4.5 21.0004H9C9.01083 21.0004 9.02166 21.0002 9.03247 21H14.9675C14.9783 21.0002 14.9892 21.0004 15 21.0004H19.5009C19.5118 21.0004 19.5226 21.0002 19.5334 21H22.5C22.9142 21 23.25 20.6642 23.25 20.25C23.25 19.8358 22.9142 19.5 22.5 19.5H21.0009V10.8322C21.0009 10.6231 20.9572 10.4164 20.8726 10.2252C20.7891 10.0367 20.6677 9.86733 20.5159 9.72782L13.0203 2.64995L13.0102 2.64071C12.734 2.38932 12.3739 2.25 12.0005 2.25ZM19.5009 19.5V10.8323L19.4904 10.8225L12.0005 3.75017L4.51054 10.8225L4.5 10.8323V19.5H9V15.0004C9 14.6025 9.15804 14.221 9.43934 13.9397C9.72065 13.6584 10.1022 13.5004 10.5 13.5004H13.5C13.8978 13.5004 14.2794 13.6584 14.5607 13.9397C14.842 14.221 15 14.6025 15 15.0004V19.5H19.5009ZM13.5 19.5V15.0004L10.5 15.0004L10.5 19.5H13.5Z"
      />
    </Icon>
  )
);
