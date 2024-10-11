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

export enum NavigationCategory {
  Resources = 'Resources',
  Access = 'Access',
  Identity = 'Identity',
  Policy = 'Policy',
  Audit = 'Audit',
  AddNew = 'Add New',
}

export const NAVIGATION_CATEGORIES = [
  NavigationCategory.Resources,
  NavigationCategory.Access,
  NavigationCategory.Identity,
  NavigationCategory.Policy,
  NavigationCategory.Audit,
  NavigationCategory.AddNew,
];

export const STANDALONE_CATEGORIES = [
  NavigationCategory.AddNew,
  // TODO(rudream): Remove this once shortcuts to pinned/nodes/apps/dbs/desktops/kubes are implemented.
  NavigationCategory.Resources,
];
