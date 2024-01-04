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

export function SettingsIcon({ size = 32, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 32 32">
      <path d="M29.181 19.07c-1.679-2.908-.669-6.634 2.255-8.328l-3.145-5.447a6.022 6.022 0 0 1-3.058.829c-3.361 0-6.085-2.742-6.085-6.125h-6.289a6.023 6.023 0 0 1-.811 3.07C10.369 5.977 6.637 6.966 3.709 5.28L.565 10.727a6.023 6.023 0 0 1 2.246 2.234c1.676 2.903.672 6.623-2.241 8.319l3.145 5.447a6.022 6.022 0 0 1 3.044-.82c3.35 0 6.067 2.725 6.084 6.092h6.289a6.032 6.032 0 0 1 .811-3.038c1.676-2.903 5.399-3.894 8.325-2.219l3.145-5.447a6.032 6.032 0 0 1-2.232-2.226zM16 22.479A6.48 6.48 0 1 1 16 9.52a6.48 6.48 0 0 1 0 12.959z" />
    </SVGIcon>
  );
}
