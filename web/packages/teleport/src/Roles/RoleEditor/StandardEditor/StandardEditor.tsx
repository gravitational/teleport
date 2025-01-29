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

import { useCallback, useId, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import { SlideTabs } from 'design/SlideTabs';
import { useValidation } from 'shared/components/Validation';

import { Role, RoleWithYaml } from 'teleport/services/resources';

import { EditorSaveCancelButton } from '../Shared';
import { AccessRules } from './AccessRules';
import { MetadataSection } from './MetadataSection';
import { Options } from './Options';
import { RequiresResetToStandard } from './RequiresResetToStandard';
import { ResourcesTab } from './Resources';
import {
  OptionsModel,
  roleEditorModelToRole,
  StandardEditorModel,
} from './standardmodel';
import { StandardModelDispatcher } from './useStandardModel';

export type StandardEditorProps = {
  originalRole: RoleWithYaml;
  standardEditorModel: StandardEditorModel;
  isProcessing?: boolean;
  onSave?(r: Role): void;
  onCancel?(): void;
  dispatch: StandardModelDispatcher;
};

/**
 * A structured editor that represents a role with a series of UI controls, as
 * opposed to a YAML text editor.
 */
export const StandardEditor = ({
  originalRole,
  standardEditorModel,
  isProcessing,
  onSave,
  onCancel,
  dispatch,
}: StandardEditorProps) => {
  const isEditing = !!originalRole;
  const { roleModel, validationResult } = standardEditorModel;

  enum StandardEditorTab {
    Overview,
    Resources,
    AccessRules,
    Options,
  }

  const [currentTab, setCurrentTab] = useState(StandardEditorTab.Overview);
  const idPrefix = useId();
  const overviewTabId = `${idPrefix}-overview`;
  const resourcesTabId = `${idPrefix}-resources`;
  const accessRulesTabId = `${idPrefix}-access-rules`;
  const optionsTabId = `${idPrefix}-options`;

  const validator = useValidation();

  function handleSave() {
    if (!validator.validate()) {
      return;
    }
    onSave?.(roleEditorModelToRole(standardEditorModel.roleModel));
  }

  const setOptions = useCallback(
    (options: OptionsModel) =>
      dispatch({ type: 'set-options', payload: options }),
    [dispatch]
  );

  return (
    <>
      {roleModel.requiresReset && (
        <Box mx={3}>
          <RequiresResetToStandard />
        </Box>
      )}
      <EditorWrapper
        mute={standardEditorModel.roleModel.requiresReset}
        data-testid="standard-editor"
      >
        <Box mb={3} mx={3}>
          <SlideTabs
            appearance="round"
            hideStatusIconOnActiveTab
            tabs={[
              {
                key: StandardEditorTab.Overview,
                title: 'Overview',
                controls: overviewTabId,
                status:
                  validator.state.validating && !validationResult.metadata.valid
                    ? validationErrorTabStatus
                    : undefined,
              },
              {
                key: StandardEditorTab.Resources,
                title: 'Resources',
                controls: resourcesTabId,
                status:
                  validator.state.validating &&
                  validationResult.resources.some(s => !s.valid)
                    ? validationErrorTabStatus
                    : undefined,
              },
              {
                key: StandardEditorTab.AccessRules,
                title: 'Access Rules',
                controls: accessRulesTabId,
                status:
                  validator.state.validating &&
                  validationResult.rules.some(s => !s.valid)
                    ? validationErrorTabStatus
                    : undefined,
              },
              {
                key: StandardEditorTab.Options,
                title: 'Options',
                controls: optionsTabId,
              },
            ]}
            activeIndex={currentTab}
            onChange={setCurrentTab}
          />
        </Box>
        <Flex
          flex="1 1 0"
          flexDirection="column"
          px={3}
          pb={3}
          css={`
            overflow-y: auto;
          `}
        >
          <Box
            id={overviewTabId}
            style={{
              display: currentTab === StandardEditorTab.Overview ? '' : 'none',
            }}
          >
            <MetadataSection
              value={roleModel.metadata}
              isProcessing={isProcessing}
              validation={validationResult.metadata}
              onChange={metadata =>
                dispatch({ type: 'set-metadata', payload: metadata })
              }
            />
          </Box>
          <Box
            id={resourcesTabId}
            style={{
              display: currentTab === StandardEditorTab.Resources ? '' : 'none',
            }}
          >
            <ResourcesTab
              value={roleModel.resources}
              isProcessing={isProcessing}
              validation={validationResult.resources}
              dispatch={dispatch}
            />
          </Box>
          <Box
            id={accessRulesTabId}
            style={{
              display:
                currentTab === StandardEditorTab.AccessRules ? '' : 'none',
            }}
          >
            <AccessRules
              isProcessing={isProcessing}
              value={roleModel.rules}
              dispatch={dispatch}
              validation={validationResult.rules}
            />
          </Box>
          <Box
            id={optionsTabId}
            style={{
              display: currentTab === StandardEditorTab.Options ? '' : 'none',
            }}
          >
            <Options
              isProcessing={isProcessing}
              value={roleModel.options}
              onChange={setOptions}
            />
          </Box>
        </Flex>
      </EditorWrapper>
      <EditorSaveCancelButton
        onSave={() => handleSave()}
        onCancel={onCancel}
        saveDisabled={
          isProcessing ||
          standardEditorModel.roleModel.requiresReset ||
          !standardEditorModel.isDirty
        }
        isEditing={isEditing}
      />
    </>
  );
};

const validationErrorTabStatus = {
  kind: 'danger',
  ariaLabel: 'Invalid data',
} as const;

export const EditorWrapper = styled(Flex)<{ mute?: boolean }>`
  flex-direction: column;
  flex: 1;
  opacity: ${p => (p.mute ? 0.4 : 1)};
  pointer-events: ${p => (p.mute ? 'none' : '')};
`;
