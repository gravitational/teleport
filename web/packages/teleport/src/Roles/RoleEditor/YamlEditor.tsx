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

import { useState } from 'react';

import { Flex } from 'design';
import TextEditor from 'shared/components/TextEditor';

import { RoleWithYaml } from 'teleport/services/resources';

import { ActionButtonsContainer, PreviewButton, SaveButton } from './Shared';
import { YamlEditorModel } from './yamlmodel';

type YamlEditorProps = {
  originalRole: RoleWithYaml;
  yamlEditorModel: YamlEditorModel;
  isProcessing: boolean;
  onChange?(y: YamlEditorModel): void;
  onSave?(content: string): void;
  onPreview?(): void;
};

export const YamlEditor = ({
  originalRole,
  isProcessing,
  yamlEditorModel,
  onChange,
  onSave,
  onPreview,
}: YamlEditorProps) => {
  const isEditing = !!originalRole;
  const [wasPreviewed, setHasPreviewed] = useState(!onPreview);

  const handleSave = () => onSave?.(yamlEditorModel.content);

  const handlePreview = () => {
    // handlePreview should only be called if `onPreview` exists, but adding
    // the extra safety here to protect against potential misuse
    onPreview?.();
    setHasPreviewed(true);
  };

  function handleSetYaml(newContent: string) {
    if (onPreview) {
      setHasPreviewed(false);
    }
    onChange?.({
      isDirty: originalRole?.yaml !== newContent,
      content: newContent,
    });
  }

  return (
    <Flex flexDirection="column" flex="1" data-testid="yaml-editor">
      <Flex flex="1" px={3} data-testid="text-editor-container">
        <TextEditor
          readOnly={isProcessing}
          data={[{ content: yamlEditorModel.content, type: 'yaml' }]}
          onChange={handleSetYaml}
        />
      </Flex>
      <ActionButtonsContainer>
        <SaveButton
          isEditing={isEditing}
          disabled={isProcessing || !yamlEditorModel.isDirty || !wasPreviewed}
          onClick={handleSave}
        />
        {onPreview && (
          <PreviewButton
            disabled={isProcessing || wasPreviewed || !yamlEditorModel.isDirty}
            onClick={handlePreview}
          />
        )}
      </ActionButtonsContainer>
    </Flex>
  );
};
