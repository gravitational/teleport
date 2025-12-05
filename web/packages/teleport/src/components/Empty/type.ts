/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { ResourceAccessKind } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';

/**
 * awsIcApp describes a sub_kind of application and some features
 * require an empty state for it e.g. access list.
 */
export type EmptyResourceKind = ResourceAccessKind | 'awsIcApp';

type Base = {
  canCreate: boolean;
  clusterId: string;
};

/**
 * Custom allows you to customize the empty state info.
 */
export type Custom = Base & {
  emptyStateInfo: EmptyStateInfo;
  kind?: never;
};

/**
 * SingleResource uses default empty state info specific
 * to the resource kind specified.
 */
export type SingleResource = Base & {
  kind: EmptyResourceKind;
  emptyStateInfo?: never;
};

export type EmptyStateInfo = {
  byline: string;
  docsURL?: string;
  readOnly: {
    title: string;
    resource: string;
  };
  title: string;
};
