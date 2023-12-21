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

import React from 'react';

import { SVGIcon } from './SVGIcon';

import type { SVGIconProps } from './common';

export function SpaceliftIcon({ size = 80, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 76.56 65.65" size={size} fill={fill}>
      <path d="M60.79 27.92c9.98-3.34 16.27-7.29 15.74-10.52-.52-3.19-7.56-4.93-17.86-4.98-5.89-11.3-19.82-15.69-31.12-9.81a23.07 23.07 0 0 0-12.13 16.83C5.65 22.76-.49 26.65.03 29.85c.53 3.24 7.75 4.99 18.29 4.99 1.68 2.84 3.94 5.28 6.64 7.18a34.968 34.968 0 0 0-8.59 23.67h5.99a29.249 29.249 0 0 1 6.23-18.67c-.9 6.19-1.46 12.42-1.69 18.67h10.63s-1.95-8.06 1.22-8.07c3.18 0 1.22 8.07 1.22 8.07h10.64c-.21-5.9-.73-11.79-1.55-17.65 1.18.95 2.51 1.71 3.94 2.24 1.64.61 3.38.87 5.13.75 3.17-.24 6.11-1.78 8.09-4.27 2.24-2.67 3.69-6.42 4.31-11.14l-5.94-.78c-.82 6.24-3.33 9.96-6.89 10.22a7.81 7.81 0 0 1-6.24-3.04c4.76-3.36 8.09-8.39 9.31-14.09ZM1.51 29.61c-.16-1 1.6-3.56 9.23-6.79 1.39-.59 2.89-1.16 4.47-1.71-.17 1.9-.11 3.81.19 5.7 1.11 6.89 46.79-.16 45.62-7.38-.31-1.89-.86-3.74-1.63-5.5 11.02.14 15.42 2.27 15.66 3.71.16 1-1.6 3.56-9.24 6.79-6.83 2.89-16.16 5.39-26.26 7.03-7.01 1.18-14.11 1.8-21.22 1.87-11.9 0-16.58-2.23-16.82-3.73Z" />
    </SVGIcon>
  );
}
