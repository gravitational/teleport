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

export const FlowArrow = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-flowarrow"
      {...otherProps}
      ref={ref}
    >
      <path d="M18.97 3.97a.75.75 0 0 1 1.06 0l2.992 2.991a.75.75 0 0 1 .008 1.07l-3 3a.75.75 0 1 1-1.06-1.061l1.764-1.765-.306-.01c-.696-.021-1.38-.03-2.041.015-1.327.09-2.488.389-3.408 1.153-.794.661-1.104 1.558-1.437 2.695l-.056.193c-.303 1.044-.666 2.295-1.647 3.278-.922.925-2.018 1.35-2.858 1.547-.3.07-.572.112-.799.138a3.751 3.751 0 1 1-.017-1.51q.215-.029.474-.088c.667-.157 1.475-.481 2.138-1.146.702-.705.968-1.612 1.3-2.743l.026-.09c.327-1.116.732-2.441 1.918-3.427 1.266-1.053 2.793-1.397 4.265-1.497a23 23 0 0 1 2.354-.012l-1.67-1.67a.75.75 0 0 1 0-1.061M6.75 16.5a2.25 2.25 0 1 0-4.5 0 2.25 2.25 0 0 0 4.5 0" />
    </Icon>
  )
);
