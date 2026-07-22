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

export type UnifiedInstance = {
  id: string;
  type: 'instance' | 'bot_instance';
  instance?: Instance;
  botInstance?: BotInstance;
};

export type Instance = {
  name: string;
  version: string;
  services: string[];
  upgrader?: UpgraderInfo;
};

export type BotInstance = {
  name: string;
  version: string;
};

export type UpgraderInfo = {
  type: string;
  version: string;
  group: string;
};

export type UnifiedInstancesResponse = {
  instances: UnifiedInstance[];
  startKey?: string;
};
