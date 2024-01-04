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

export function DisplayIcon({ size = 32, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 32 32">
      <path d="M30.662 9.003a98.793 98.793 0 0 0-8.815-.851L27 2.999l-2-2-7.017 7.017c-.656-.011-1.317-.017-1.983-.017l-8-8-2 2 6.069 6.069c-3.779.133-7.386.454-10.731.935C.478 12.369 0 16.089 0 20s.477 7.63 1.338 10.997C5.827 31.642 10.786 32 16 32s10.173-.358 14.662-1.003C31.522 27.631 32 23.911 32 20s-.477-7.63-1.338-10.997zm-3.665 18.328C23.63 27.761 19.911 28 16 28s-7.63-.239-10.997-.669C4.358 25.087 4 22.607 4 20s.358-5.087 1.003-7.331C8.369 12.239 12.089 12 16 12s7.63.239 10.996.669C27.641 14.913 28 17.393 28 20s-.358 5.087-1.003 7.331z" />
    </SVGIcon>
  );
}
