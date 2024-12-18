/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Cell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import { BotOptionsCellProps } from 'teleport/Bots/types';
import { BotUiFlow } from 'teleport/services/bot/types';

export function BotOptionsCell({
  onClickDelete,
  onClickView,
  bot,
  disabledEdit,
  disabledDelete,
  onClickEdit,
}: BotOptionsCellProps) {
  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem onClick={onClickEdit} disabled={disabledEdit}>
          Edit...
        </MenuItem>
        <MenuItem onClick={onClickDelete} disabled={disabledDelete}>
          Delete...
        </MenuItem>
        {bot.type === BotUiFlow.GitHubActionsSsh && (
          <MenuItem onClick={onClickView}>View...</MenuItem>
        )}
      </MenuButton>
    </Cell>
  );
}
