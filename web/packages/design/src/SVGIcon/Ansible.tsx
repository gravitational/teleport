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

export function AnsibleIcon({ size = 100, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 24 24" size={size} fill={fill}>
      <path d="m10.617 11.473 4.686 3.695-3.102-7.662zM12 0C5.371 0 0 5.371 0 12s5.371 12 12 12 12-5.371 12-12S18.629 0 12 0zm5.797 17.305a.851.851 0 0 1-.875.83c-.236 0-.416-.09-.664-.293l-6.19-5-2.079 5.203H6.191L11.438 5.44a.79.79 0 0 1 .764-.506.756.756 0 0 1 .742.506l4.774 11.494c.045.111.08.234.08.348l-.001.023z" />
    </SVGIcon>
  );
}
