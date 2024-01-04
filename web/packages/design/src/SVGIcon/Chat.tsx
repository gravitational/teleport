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

export function ChatIcon({ size = 22, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 48 48">
      <path d="M 24 4 C 12.972292 4 4 12.972292 4 24 C 4 27.275316 4.8627078 30.334853 6.2617188 33.064453 L 4.09375 40.828125 C 3.5887973 42.631528 5.3719261 44.41261 7.1757812 43.908203 L 14.943359 41.740234 C 17.671046 43.137358 20.726959 44 24 44 C 35.027708 44 44 35.027708 44 24 C 44 12.972292 35.027708 4 24 4 z M 24 7 C 33.406292 7 41 14.593708 41 24 C 41 33.406292 33.406292 41 24 41 C 20.997029 41 18.192258 40.218281 15.744141 38.853516 A 1.50015 1.50015 0 0 0 14.609375 38.71875 L 7.2226562 40.78125 L 9.2851562 33.398438 A 1.50015 1.50015 0 0 0 9.1503906 32.263672 C 7.7836522 29.813476 7 27.004518 7 24 C 7 14.593708 14.593708 7 24 7 z" />
    </SVGIcon>
  );
}
