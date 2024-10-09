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

import React, { useState } from 'react';
import { Box, Flex, H3 } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import * as Icon from 'design/Icon';
import { ToolTipInfo } from 'shared/components/ToolTip';
import styled, { useTheme } from 'styled-components';

import { Role, RoleWithYaml } from 'teleport/services/resources';

import {
  roleEditorModelToRole,
  hasModifiedFields,
  MetadataModel,
  RoleEditorModel,
  StandardEditorModel,
} from './standardmodel';
import { EditorSaveCancelButton } from './Shared';
import { RequiresResetToStandard } from './RequiresResetToStandard';

type StandardEditorProps = {
  originalRole: RoleWithYaml;
  standardEditorModel: StandardEditorModel;
  isProcessing: boolean;
  onSave?(r: Role): void;
  onCancel?(): void;
  onChange?(s: StandardEditorModel): void;
};

export const StandardEditor = ({
  originalRole,
  standardEditorModel,
  isProcessing,
  onSave,
  onCancel,
  onChange,
}: StandardEditorProps) => {
  const isEditing = !!originalRole;
  const { roleModel } = standardEditorModel;

  function handleSave(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    onSave?.(roleEditorModelToRole(standardEditorModel.roleModel));
  }

  function handleChange(modified: Partial<RoleEditorModel>) {
    const updatedResourceModel: RoleEditorModel = {
      ...roleModel,
      ...modified,
    };

    onChange?.({
      ...standardEditorModel,
      roleModel: updatedResourceModel,
      isDirty: hasModifiedFields(updatedResourceModel, originalRole?.object),
    });
  }

  /**
   * Resets the standard editor back into viewable state. The existing model
   * has been already stripped from unsupported features by the parsing
   * attempt, the only thing left to do is to set the `requiresReset` flag.
   */
  function resetForStandardEditor() {
    onChange?.({
      ...standardEditorModel,
      isDirty: true,
      roleModel: {
        ...standardEditorModel.roleModel,
        requiresReset: false,
      },
    });
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          {roleModel.requiresReset && (
            <RequiresResetToStandard reset={resetForStandardEditor} />
          )}
          <EditorWrapper
            mute={standardEditorModel.roleModel.requiresReset}
            data-testid="standard-editor"
          >
            <Box my={2}>
              <MetadataSection
                value={roleModel.metadata}
                isProcessing={isProcessing}
                onChange={metadata => handleChange({ ...roleModel, metadata })}
              />
            </Box>
          </EditorWrapper>
          <EditorSaveCancelButton
            onSave={() => handleSave(validator)}
            onCancel={onCancel}
            disabled={
              isProcessing ||
              standardEditorModel.roleModel.requiresReset ||
              !standardEditorModel.isDirty
            }
            isEditing={isEditing}
          />
        </>
      )}
    </Validation>
  );
};

const MetadataSection = ({
  value,
  isProcessing,
  onChange,
}: {
  value: MetadataModel;
  isProcessing: boolean;
  onChange: (m: MetadataModel) => void;
}) => (
  <Section
    title="Role Metadata"
    tooltip="Basic information about the role resource"
  >
    <FieldInput
      label="Role Name"
      placeholder="Enter Role Name"
      value={value.name}
      disabled={isProcessing}
      rule={requiredField('Role name is required')}
      onChange={e => onChange({ ...value, name: e.target.value })}
    />
    <FieldInput
      label="Description"
      placeholder="Enter Role Description"
      value={value.description || ''}
      disabled={isProcessing}
      onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
        onChange({ ...value, description: e.target.value })
      }
    />
  </Section>
);

const Section = ({
  title,
  tooltip,
  children,
}: React.PropsWithChildren<{ title: string; tooltip?: string }>) => {
  const theme = useTheme();
  const [expanded, setExpanded] = useState(true);
  const ExpandIcon = expanded ? Icon.Minus : Icon.Plus;
  return (
    <Box
      border={1}
      borderColor={theme.colors.interactive.tonal.neutral[0].background}
      borderRadius={2}
    >
      <Flex p={3} css={'cursor: pointer'} onClick={() => setExpanded(e => !e)}>
        <Flex flex="1" gap={2}>
          <H3>{title}</H3>
          {tooltip && <ToolTipInfo>{tooltip}</ToolTipInfo>}
        </Flex>
        <ExpandIcon size={16} color={theme.colors.text.muted} />
      </Flex>
      {expanded && (
        <Box px={3} pb={3}>
          {children}
        </Box>
      )}
    </Box>
  );
};

export const EditorWrapper = styled(Box)<{ mute?: boolean }>`
  opacity: ${p => (p.mute ? 0.4 : 1)};
  pointer-events: ${p => (p.mute ? 'none' : '')};
`;
