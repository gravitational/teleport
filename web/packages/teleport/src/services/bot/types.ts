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

export type ApiBotMetadata = {
  description: string;
  labels: Map<string, string>;
  name: string;
  namespace: string;
  revision: string;
};

export type ApiBotSpec = {
  roles: string[];
  traits: ApiBotTrait[];
};

export type ApiBotTrait = {
  name: string;
  values: string[];
};

export type ApiBot = {
  kind: string;
  metadata: ApiBotMetadata;
  spec: ApiBotSpec;
  status: string;
  subKind: string;
  version: string;
};

export type BotList = {
  bots: FlatBot[];
};

export type FlatBot = Omit<ApiBot, 'metadata' | 'spec'> &
  ApiBotMetadata &
  ApiBotSpec;

export type BotResponse = {
  items: ApiBot[];
};
