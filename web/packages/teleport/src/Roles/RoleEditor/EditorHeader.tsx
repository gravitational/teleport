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

import { Box, ButtonIcon, ButtonSecondary, Flex, H2, Indicator } from 'design';
import { ArrowsIn, Cross } from 'design/Icon';

import { Role } from 'teleport/services/resources';

import { EditorTab, EditorTabs } from './EditorTabs';

/** Renders a header button with role name and delete button. */
export const EditorHeader = ({
  role = null,
  selectedEditorTab,
  onEditorTabChange,
  isProcessing,
  standardEditorId,
  yamlEditorId,
  onClose,
  minimized,
  onMinimizedChange,
}: {
  role?: Role;
  selectedEditorTab: EditorTab;
  onEditorTabChange(t: EditorTab): void;
  isProcessing: boolean;
  standardEditorId: string;
  yamlEditorId: string;
  onClose(): void;
  minimized: boolean;
  onMinimizedChange(minimized: boolean): void;
}) => {
  const isCreating = !role;
  const theme = useTheme();

  return (
    <Flex alignItems="center" mb={3} gap={2}>
      <ButtonIcon aria-label="Close" onClick={onClose}>
        <Cross size="small" />
      </ButtonIcon>
      <Box
        flex="1"
        role={minimized ? 'button' : undefined}
        onClick={minimized ? () => onMinimizedChange(false) : undefined}
        style={{ cursor: minimized ? 'pointer' : undefined }}
      >
        <H2>
          {isCreating
            ? 'Create a New Role'
            : `Edit Role ${role?.metadata.name}`}
        </H2>
      </Box>
      <Box flex="0 0 24px" lineHeight={0}>
        {isProcessing && <Indicator size={24} color="text.muted" />}
      </Box>
      {!minimized && (
        <>
          <EditorTabs
            onTabChange={onEditorTabChange}
            selectedEditorTab={selectedEditorTab}
            disabled={isProcessing}
            standardEditorId={standardEditorId}
            yamlEditorId={yamlEditorId}
          />
          <ButtonSecondary
            size="large"
            width="40px"
            px={0}
            onClick={() => onMinimizedChange(true)}
          >
            <ArrowsIn size="medium" color={theme.colors.text.slightlyMuted} />
          </ButtonSecondary>
        </>
      )}
    </Flex>
  );
};
