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

export function ChevronDownIcon({ size = 14, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 14 9" size={size} fill={fill}>
      <path d="M13.8889 1.65386L13.2629 1.02792C13.1148 0.879771 12.8746 0.879771 12.7264 1.02792L7 6.7407L1.27359 1.02795C1.12545 0.879802 0.885235 0.879802 0.737056 1.02795L0.111112 1.65389C-0.0370353 1.80204 -0.0370353 2.04225 0.111112 2.19043L6.73175 8.81101C6.8799 8.95916 7.12011 8.95916 7.26829 8.81101L13.8889 2.1904C14.037 2.04222 14.037 1.80201 13.8889 1.65386Z" />
    </SVGIcon>
  );
}
