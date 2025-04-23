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

export const Bubble = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-bubble"
      {...otherProps}
      ref={ref}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M6.06203 4.2699C7.9386 2.82839 10.2754 2.11853 12.6367 2.27267C14.9979 2.42681 17.2226 3.43444 18.8958 5.10767C20.5691 6.78091 21.5767 9.00556 21.7308 11.3669C21.885 13.7281 21.1751 16.0649 19.7336 17.9415C18.2921 19.818 16.2173 21.1063 13.8961 21.5661C11.6873 22.0037 9.39885 21.6626 7.41772 20.6073L4.22529 21.672C3.96099 21.7601 3.67736 21.7729 3.40621 21.7089C3.13505 21.6449 2.88708 21.5067 2.69007 21.3097C2.49307 21.1127 2.35483 20.8647 2.29083 20.5935C2.22684 20.3224 2.23963 20.0388 2.32777 19.7745L3.39597 16.5853C2.34083 14.6043 1.99988 12.316 2.43738 10.1074C2.89719 7.78619 4.18547 5.71142 6.06203 4.2699ZM12.5389 3.76949C10.5409 3.63906 8.56367 4.23971 6.9758 5.45945C5.38794 6.67919 4.29786 8.43477 3.90879 10.3989C3.51972 12.363 3.85823 14.4015 4.86119 16.1345C4.96898 16.3208 4.99158 16.5443 4.92323 16.7484L3.75073 20.249L7.25603 19.08C7.45985 19.012 7.68304 19.0347 7.869 19.1423C9.60196 20.1453 11.6405 20.4838 13.6046 20.0947C15.5687 19.7056 17.3243 18.6156 18.5441 17.0277C19.7638 15.4398 20.3644 13.4626 20.234 11.4646C20.1036 9.46655 19.251 7.58415 17.8352 6.16833C16.4194 4.75252 14.537 3.89991 12.5389 3.76949Z"
      />
    </Icon>
  )
);
