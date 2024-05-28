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

import { FlatBot } from 'teleport/services/bot/types';

export type BotOptionsCellProps = {
  bot: FlatBot;
  disabledEdit: boolean;
  disabledDelete: boolean;
  onClickEdit: (bot: FlatBot) => void;
  onClickDelete: (bot: FlatBot) => void;
  onClickView: (bot: FlatBot) => void;
};

export type BotListProps = {
  attempt: Attempt;
  bots: FlatBot[];
  disabledEdit: boolean;
  disabledDelete: boolean;
  fetchRoles: (input: string) => Promise<string[]>;
  onClose: () => void;
  onDelete: () => void;
  onEdit: () => void;
  selectedBot: FlatBot;
  setSelectedBot: Dispatch<SetStateAction<FlatBot>>;
  selectedRoles: string[];
  setSelectedRoles: Dispatch<SetStateAction<string[]>>;
};

export type DeleteBotProps = {
  attempt: Attempt;
  name: string;
  onClose: () => void;
  onDelete: () => void;
};

export type ViewBotProps = {
  bot: FlatBot;
  onClose: () => void;
};

export enum BotFlowType {
  GitHubActions = 'github-actions',
}

export type EditBotProps = {
  fetchRoles: (input: string) => Promise<string[]>;
  attempt: Attempt;
  name: string;
  onClose: () => void;
  onEdit: () => void;
  selectedRoles: string[];
  setSelectedRoles: Dispatch<SetStateAction<string[]>>;
};
