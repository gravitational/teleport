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

import { ServerResource } from 'teleport/Discover/Server';
import { AwsMangementConsole } from 'teleport/Discover/AwsMangementConsole';
import { DatabaseResource } from 'teleport/Discover/Database';
import { KubernetesResource } from 'teleport/Discover/Kubernetes';
import { DesktopResource } from 'teleport/Discover/Desktop';
import { ConnectMyComputerResource } from 'teleport/Discover/ConnectMyComputer';

import { ResourceViewConfig } from './flow';

export const viewConfigs: ResourceViewConfig[] = [
  AwsMangementConsole,
  ServerResource,
  DatabaseResource,
  KubernetesResource,
  DesktopResource,
  ConnectMyComputerResource,
];
