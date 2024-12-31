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

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import { SearchResource } from 'teleport/Discover/SelectResource/types';

export const resourceMapping: { [key in Resource]: SearchResource } = {
  [Resource.UNSPECIFIED]: SearchResource.UNSPECIFIED,
  [Resource.DATABASES]: SearchResource.DATABASE,
  [Resource.KUBERNETES]: SearchResource.KUBERNETES,
  [Resource.SERVER_SSH]: SearchResource.SERVER,
  [Resource.WEB_APPLICATIONS]: SearchResource.APPLICATION,
  [Resource.WINDOWS_DESKTOPS]: SearchResource.DESKTOP,
};
