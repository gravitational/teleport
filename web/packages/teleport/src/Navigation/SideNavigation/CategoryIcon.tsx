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

import { ReactNode } from 'react';

import * as Icons from 'design/Icon';

import {
  CustomNavigationCategory,
  NavigationCategory,
  SidenavCategory,
} from './categories';

export function CategoryIcon({
  category,
  size,
  color,
}: {
  category: SidenavCategory;
  size?: number;
  color?: string;
}) {
  let Icon: ({ size, color }) => ReactNode;
  switch (category) {
    case NavigationCategory.Resources:
      Icon = Icons.Server;
      break;
    case NavigationCategory.Access:
      Icon = Icons.KeyHole;
      break;
    case NavigationCategory.Identity:
      Icon = Icons.FingerprintSimple;
      break;
    case NavigationCategory.Policy:
      Icon = Icons.ShieldCheck;
      break;
    case NavigationCategory.Audit:
      Icon = Icons.ListMagnifyingGlass;
      break;
    case NavigationCategory.AddNew:
      Icon = Icons.AddCircle;
      break;
    case CustomNavigationCategory.Search:
      Icon = Icons.Magnifier;
      break;
    default:
      return null;
  }

  return <Icon size={size} color={color} />;
}
