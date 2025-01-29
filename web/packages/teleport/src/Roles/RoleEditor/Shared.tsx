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

import { useTheme } from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex } from 'design';
import { HoverTooltip } from 'design/Tooltip';

import useTeleport from 'teleport/useTeleport';

export const EditorSaveCancelButton = ({
  onSave,
  onPreview,
  onCancel,
  saveDisabled,
  previewDisabled = true,
  isEditing,
}: {
  onSave?(): void;
  onPreview?(): void;
  onCancel?(): void;
  saveDisabled: boolean;
  previewDisabled?: boolean;
  isEditing?: boolean;
}) => {
  const ctx = useTeleport();
  const roleAccess = ctx.storeUser.getRoleAccess();
  const theme = useTheme();

  let hoverTooltipContent = '';
  if (isEditing && !roleAccess.edit) {
    hoverTooltipContent = 'You do not have access to update roles';
  } else if (!isEditing && !roleAccess.create) {
    hoverTooltipContent = 'You do not have access to create roles';
  }

  const saveButton = (
    <Box width="50%">
      <HoverTooltip tipContent={hoverTooltipContent}>
        <ButtonPrimary
          width="100%"
          size="large"
          onClick={onSave}
          disabled={
            saveDisabled ||
            (isEditing && !roleAccess.edit) ||
            (!isEditing && !roleAccess.create)
          }
        >
          {isEditing ? 'Save Changes' : 'Create Role'}
        </ButtonPrimary>
      </HoverTooltip>
    </Box>
  );
  const cancelButton = (
    <ButtonSecondary width="50%" onClick={onCancel}>
      Cancel
    </ButtonSecondary>
  );

  const previewButton = (
    <ButtonPrimary width="50%" onClick={onPreview} disabled={previewDisabled}>
      Preview
    </ButtonPrimary>
  );

  return (
    <Flex
      gap={2}
      p={3}
      borderTop={1}
      borderColor={theme.colors.interactive.tonal.neutral[0]}
    >
      {saveButton}
      {onPreview ? previewButton : cancelButton}
    </Flex>
  );
};
