/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import * as Icons from 'design/Icon';

import { CustomNavigationCategory, NavigationCategory } from './categories';

export function CategoryIcon({
  category,
  size,
  color,
}: {
  category: NavigationCategory | CustomNavigationCategory;
  size?: number;
  color?: string;
}) {
  switch (category) {
    case NavigationCategory.Resources:
      return <Icons.Server size={size} color={color} />;
    case NavigationCategory.Access:
      return <Icons.Lock size={size} color={color} />;
    case NavigationCategory.Identity:
      return <Icons.FingerprintSimple size={size} color={color} />;
    case NavigationCategory.Policy:
      return <Icons.ShieldCheck size={size} color={color} />;
    case NavigationCategory.Audit:
      return <Icons.ListMagnifyingGlass size={size} color={color} />;
    case NavigationCategory.AddNew:
      return <Icons.AddCircle size={size} color={color} />;
    case CustomNavigationCategory.Search:
      return <Icons.Magnifier size={size} color={color} />;
    default:
      return null;
  }
}
