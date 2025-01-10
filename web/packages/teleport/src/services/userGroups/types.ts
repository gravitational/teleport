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

import { ResourceLabel } from 'teleport/services/agents';

// UserGroup is a remote user group.
export type UserGroup = {
  kind: 'user_group';
  // Name is name of the user group.
  name: string;
  // Description is the description of the user group.
  description: string;
  // Labels for the user group.
  labels: ResourceLabel[];
  // FriendlyName for the user group.
  friendlyName?: string;
  // Applications is a list of associated applications.
  applications?: ApplicationAndFriendlyName[];
  // userGroups won't ever have this flag, but the key is added here
  // so it can appear on the common 'UnifiedResource' type
  requiresRequest?: false;
};

export type ApplicationAndFriendlyName = {
  name: string;
  friendlyName: string;
};

export type UserGroupsResponse = {
  userGroups: UserGroup[];
  startKey?: string;
  totalCount?: number;
};
