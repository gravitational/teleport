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

/**
 * CustomNavigationCategory are pseudo-categories which exist only in the nav menu, eg. Search.
 */
export enum CustomNavigationCategory {
  Search = 'Search',
}

/**
 * CustomNavigationSubcategory are subcategories within a navigation category which can be used to
 * create groupings of subsections, eg. Filtered Views.
 */
export enum CustomNavigationSubcategory {
  FilteredViews = 'Filtered Views',
}

export type SidenavCategory = NavigationCategory | CustomNavigationCategory;

export const NAVIGATION_CATEGORIES = [
  NavigationCategory.Access,
  NavigationCategory.Identity,
  NavigationCategory.Policy,
  NavigationCategory.Audit,
  NavigationCategory.AddNew,
];
