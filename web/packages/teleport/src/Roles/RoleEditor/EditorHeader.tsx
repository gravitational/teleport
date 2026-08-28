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

import styled, { useTheme } from 'styled-components';

import {
  Box,
  ButtonIcon,
  ButtonSecondary,
  ButtonText,
  Flex,
  H2,
  Indicator,
} from 'design';
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
  /**
   * Called when the minimize button is clicked (in maximized state) or when
   * the header itself is clicked (in minimized state). If `undefined`, the
   * minimize button won't be rendered.
   */
  onMinimizedChange?(minimized: boolean): void;
}) => {
  const isCreating = !role;
  const theme = useTheme();

  const heading = (
    <H2>
      {isCreating ? 'Create a New Role' : `Edit Role ${role?.metadata.name}`}
    </H2>
  );

  return (
    <Flex alignItems="center" mb={3} gap={2}>
      <ButtonIcon aria-label="Close" onClick={onClose}>
        <Cross size="small" />
      </ButtonIcon>
      <Box flex="1">
        {minimized ? (
          // If minimized, we wrap the heading in a button that maximizes the
          // editor.
          <HeadingButton block onClick={() => onMinimizedChange?.(false)}>
            {heading}
          </HeadingButton>
        ) : (
          heading
        )}
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
          {onMinimizedChange && (
            <ButtonSecondary
              size="large"
              width="40px"
              px={0}
              onClick={() => onMinimizedChange(true)}
            >
              <ArrowsIn size="medium" color={theme.colors.text.slightlyMuted} />
            </ButtonSecondary>
          )}
        </>
      )}
    </Flex>
  );
};

const HeadingButton = styled(ButtonText)`
  padding: 0;
  justify-content: start;
  color: inherit;
  // Since we align contents to left and want to preserve the existing layout,
  // we nudge the element a bit to the left and give it a matching left padding
  // to give some breathing room between the edge and the text (which is
  // visible in the hover and focus states).
  margin-left: -${props => props.theme.space[2]}px;
  padding-left: ${props => props.theme.space[2]}px;
`;
