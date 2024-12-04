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

import React from 'react';
import { Flex, ButtonText, H2 } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import { Trash } from 'design/Icon';

import useTeleport from 'teleport/useTeleport';
import { Role } from 'teleport/services/resources';

/** Renders a header button with role name and delete button. */
export const EditorHeader = ({
  role = null,
  onDelete,
}: {
  onDelete?(): void;
  role?: Role;
}) => {
  const ctx = useTeleport();
  const isCreating = !role;

  const hasDeleteAccess = ctx.storeUser.getRoleAccess().remove;

  return (
    <Flex alignItems="center" mb={3} justifyContent="space-between">
      <H2>{isCreating ? 'Create a New Role' : role?.metadata.name}</H2>
      {!isCreating && (
        <HoverTooltip
          position="bottom"
          tipContent={
            hasDeleteAccess
              ? 'Delete'
              : 'You do not have access to delete a role'
          }
        >
          <ButtonText
            onClick={onDelete}
            disabled={!hasDeleteAccess}
            data-testid="delete"
            p={1}
          >
            <Trash size="medium" />
          </ButtonText>
        </HoverTooltip>
      )}
    </Flex>
  );
};
