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

import { NavigationCategory } from './categories';

export function CategoryIcon({ category }: { category: NavigationCategory }) {
  switch (category) {
    case NavigationCategory.Resources:
      return <Icons.Server />;
    case NavigationCategory.Access:
      return <Icons.Lock />;
    case NavigationCategory.Identity:
      return <Icons.FingerprintSimple />;
    case NavigationCategory.Policy:
      return <Icons.ShieldCheck />;
    case NavigationCategory.Audit:
      return <Icons.ListMagnifyingGlass />;
    case NavigationCategory.AddNew:
      return <Icons.AddCircle />;
    default:
      return null;
  }
}
