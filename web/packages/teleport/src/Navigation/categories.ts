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
  Management = 'Management',
}

export enum ManagementSection {
  Access = 'Access Management',
  Identity = 'Identity',
  Activity = 'Activity',
  Billing = 'Usage & Billing',
  Clusters = 'Clusters',
  Permissions = 'Permissions Management',
}

export const MANAGEMENT_NAVIGATION_SECTIONS = [
  ManagementSection.Access,
  ManagementSection.Permissions,
  ManagementSection.Identity,
  ManagementSection.Activity,
  ManagementSection.Billing,
  ManagementSection.Clusters,
];

export const NAVIGATION_CATEGORIES = [
  NavigationCategory.Resources,
  NavigationCategory.Management,
];
