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

import { ClusterResource } from 'teleport/services/userPreferences/types';
import { SearchResource } from 'teleport/Discover/SelectResource/types';

export const resourceMapping: { [key in ClusterResource]: SearchResource } = {
  [ClusterResource.RESOURCE_UNSPECIFIED]: SearchResource.UNSPECIFIED,
  [ClusterResource.RESOURCE_DATABASES]: SearchResource.DATABASE,
  [ClusterResource.RESOURCE_KUBERNETES]: SearchResource.KUBERNETES,
  [ClusterResource.RESOURCE_SERVER_SSH]: SearchResource.SERVER,
  [ClusterResource.RESOURCE_WEB_APPLICATIONS]: SearchResource.APPLICATION,
  [ClusterResource.RESOURCE_WINDOWS_DESKTOPS]: SearchResource.DESKTOP,
};
