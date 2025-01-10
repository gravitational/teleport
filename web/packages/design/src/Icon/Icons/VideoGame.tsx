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

export const VideoGame = forwardRef<HTMLSpanElement, IconProps>(
  ({ size = 24, color, ...otherProps }, ref) => (
    <Icon
      size={size}
      color={color}
      className="icon icon-videogame"
      {...otherProps}
      ref={ref}
    >
      <path d="M9 8.25C9 7.83579 8.66421 7.5 8.25 7.5C7.83579 7.5 7.5 7.83579 7.5 8.25V9H6.75C6.33579 9 6 9.33579 6 9.75C6 10.1642 6.33579 10.5 6.75 10.5H7.5V11.25C7.5 11.6642 7.83579 12 8.25 12C8.66421 12 9 11.6642 9 11.25V10.5H9.75C10.1642 10.5 10.5 10.1642 10.5 9.75C10.5 9.33579 10.1642 9 9.75 9H9V8.25Z" />
      <path d="M13.5 9.75C13.5 9.33579 13.8358 9 14.25 9H16.5C16.9142 9 17.25 9.33579 17.25 9.75C17.25 10.1642 16.9142 10.5 16.5 10.5H14.25C13.8358 10.5 13.5 10.1642 13.5 9.75Z" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M7.86299 3.75L7.86465 3.75H16.125C17.6168 3.75 19.0475 4.34263 20.1024 5.39753C20.9281 6.22321 21.4706 7.27916 21.6674 8.41444L23.1986 16.2894C23.323 16.9962 23.2189 17.7242 22.9014 18.3678C22.5839 19.0114 22.0696 19.5371 21.433 19.8684C20.7965 20.1998 20.0709 20.3197 19.3616 20.2106C18.6523 20.1015 17.9962 19.7692 17.4887 19.2618C17.4774 19.2506 17.4666 19.2391 17.4561 19.2272L13.7321 15H10.2678L6.54398 19.227C6.5335 19.2389 6.52264 19.2505 6.51142 19.2617C6.00386 19.769 5.34779 20.1014 4.6385 20.2104C3.9292 20.3195 3.2036 20.1997 2.56705 19.8683C1.93051 19.5369 1.41616 19.0113 1.09869 18.3677C0.781228 17.7241 0.677175 16.9961 0.801643 16.2893L0.803948 16.2762L2.33656 8.39453C2.56636 7.09633 3.24488 5.92 4.25375 5.07105C5.26441 4.2206 6.54212 3.75293 7.86299 3.75ZM15.7312 15L18.5646 18.2163C18.8444 18.4895 19.2027 18.6685 19.5896 18.728C19.9836 18.7886 20.3868 18.722 20.7404 18.5379C21.094 18.3538 21.3798 18.0618 21.5561 17.7043C21.7316 17.3485 21.7897 16.9464 21.7222 16.5556L20.9032 12.3431C20.6776 12.7063 20.4096 13.0453 20.1024 13.3525C19.0475 14.4074 17.6168 15 16.125 15H15.7312ZM20.1839 8.63936C20.1856 8.65107 20.1876 8.6628 20.1898 8.67454L20.1917 8.68429C20.2302 8.91101 20.25 9.14199 20.25 9.375C20.25 10.469 19.8154 11.5182 19.0418 12.2918C18.2682 13.0654 17.219 13.5 16.125 13.5H9.92903C9.71363 13.5 9.50864 13.5926 9.36626 13.7542L5.4355 18.2161C5.15572 18.4893 4.79736 18.6684 4.4105 18.7279C4.01644 18.7885 3.61333 18.7219 3.25969 18.5378C2.90606 18.3537 2.62031 18.0617 2.44394 17.7041C2.26847 17.3484 2.21035 16.9462 2.27786 16.5555L3.81024 8.6744L3.81271 8.66106C3.98035 7.70704 4.47839 6.84244 5.21954 6.21877C5.96048 5.59527 6.89716 5.25234 7.86551 5.25H16.125C17.219 5.25 18.2682 5.6846 19.0418 6.45819C19.6397 7.05611 20.0351 7.8187 20.1839 8.63936Z"
      />
    </Icon>
  )
);
