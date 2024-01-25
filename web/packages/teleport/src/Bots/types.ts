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

import { Dispatch, SetStateAction } from 'react';
import { Attempt } from 'shared/hooks/useAttemptNext';

export type BotsResponse = {
  bots: FlatBot[];
  startKey?: string;
  totalCount?: number;
};

export type BotMetadata = {
  name: string;
  namespace: string;
  description: string;
  labels: Map<string, string>;
  revision: string;
};

export type BotSpec = {
  roles: string[];
  traits: BotTrait[];
};

export type BotTrait = {
  name: string;
  values: string[];
};

export type Bot = {
  kind: string;
  metadata: BotMetadata;
  spec: BotSpec;
  status: string;
  subKind: string;
  version: string;
};

export type FlatBot = Omit<Bot, 'metadata' | 'spec'> & BotMetadata & BotSpec;

export type BotListParams = {
  bots: FlatBot[];
};

export enum BotType {
  GitHubActions = 'github-actions',
}

export type BotOptionsCellProps = {
  bot: FlatBot;
  onClickDelete: (bot: FlatBot) => void;
};

export type BotListProps = {
  attempt: Attempt;
  bots: FlatBot[];
  onClose: () => void;
  onDelete: () => void;
  selectedBot: FlatBot;
  setSelectedBot: Dispatch<SetStateAction<FlatBot>>;
};

export type DeleteBotProps = {
  attempt: Attempt;
  name: string;
  onClose: () => void;
  onDelete: () => void;
};
