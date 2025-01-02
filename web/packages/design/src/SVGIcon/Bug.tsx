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

import type { SVGIconProps } from './common';
import { SVGIcon } from './SVGIcon';

export function BugIcon({ size = 32, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 32 32">
      <path d="M32 18v-2h-6.04c-.183-2.271-.993-4.345-2.24-6.008h5.061l2.189-8.758-1.94-.485-1.811 7.242H21.76l-.084-.064c.21-.609.324-1.263.324-1.944 0-3.305-2.686-5.984-6-5.984s-6 2.679-6 5.984c0 .68.114 1.334.324 1.944l-.084.064H4.781L2.97.749l-1.94.485 2.189 8.758H8.28C7.034 11.655 6.224 13.728 6.04 16H0v2h6.043a11.782 11.782 0 0 0 1.051 3.992H3.219L1.03 30.749l1.94.485 1.811-7.243h3.511c1.834 2.439 4.606 3.992 7.708 3.992s5.874-1.554 7.708-3.992h3.511l1.811 7.243 1.94-.485-2.189-8.757h-3.875A11.76 11.76 0 0 0 25.957 18H32z" />
    </SVGIcon>
  );
}
