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

import { produce } from 'immer';
import { useCallback, useId, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex } from 'design';
import { SlideTabs } from 'design/SlideTabs';
import { TabSpec } from 'design/SlideTabs/SlideTabs';
import { useValidation } from 'shared/components/Validation';

import { Role, RoleWithYaml } from 'teleport/services/resources';

import { ActionButtonsContainer, SaveButton } from '../Shared';
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
  originalRole?: RoleWithYaml;
  standardEditorModel: StandardEditorModel;
  isProcessing?: boolean;
  onSave?(r: Role): void;
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
  const [disabledTabs, setDisabledTabs] = useState(
    isEditing ? [false, false, false, false] : [false, true, true, true]
  );
  const idPrefix = useId();

  const validator = useValidation();

  function handleResetToStandard() {
    dispatch({ type: 'reset-to-standard' });
  }

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

  const validateAndGoToNextTab = useCallback(() => {
    const nextTabIndex = currentTab + 1;
    const valid = validator.validate();
    if (!valid) {
      return;
    }
    validator.reset();
    setCurrentTab(nextTabIndex);
    setDisabledTabs(prevEnabledTabs =>
      produce(prevEnabledTabs, et => {
        et[nextTabIndex] = false;
      })
    );
  }, [currentTab, setCurrentTab, setDisabledTabs, validator]);

  const goToPreviousTab = useCallback(
    () => setCurrentTab(currentTab - 1),
    [setCurrentTab, currentTab]
  );

  const tabTitles = ['Overview', 'Resources', 'Access Rules', 'Options'];
  const tabElementIDs = [
    `${idPrefix}-overview`,
    `${idPrefix}-resources`,
    `${idPrefix}-access-rules`,
    `${idPrefix}-options`,
  ];

  function tabSpec(tab: StandardEditorTab, error: boolean): TabSpec {
    return {
      key: tab,
      title: tabTitles[tab],
      disabled: disabledTabs[tab],
      controls: tabElementIDs[tab],
      status: error ? validationErrorTabStatus : undefined,
    };
  }

  return (
    <>
      {roleModel.conversionErrors.length > 0 && (
        <Box mx={3}>
          <RequiresResetToStandard
            conversionErrors={roleModel.conversionErrors}
            onReset={handleResetToStandard}
          />
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
              tabSpec(
                StandardEditorTab.Overview,
                validator.state.validating && !validationResult.metadata.valid
              ),
              tabSpec(
                StandardEditorTab.Resources,
                validator.state.validating &&
                  validationResult.resources.some(s => !s.valid)
              ),
              tabSpec(
                StandardEditorTab.AccessRules,
                validator.state.validating &&
                  validationResult.rules.some(s => !s.valid)
              ),
              tabSpec(StandardEditorTab.Options, false),
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
            id={tabElementIDs[StandardEditorTab.Overview]}
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
            id={tabElementIDs[StandardEditorTab.Resources]}
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
            id={tabElementIDs[StandardEditorTab.AccessRules]}
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
            id={tabElementIDs[StandardEditorTab.Options]}
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
      <ActionButtonsContainer>
        {isEditing || currentTab === StandardEditorTab.Options ? (
          <SaveButton
            onClick={() => handleSave()}
            disabled={
              isProcessing ||
              standardEditorModel.roleModel.requiresReset ||
              !standardEditorModel.isDirty
            }
            isEditing={isEditing}
          />
        ) : (
          <ButtonPrimary
            size="large"
            width="50%"
            onClick={validateAndGoToNextTab}
          >
            Next: {tabTitles[currentTab + 1]}
          </ButtonPrimary>
        )}
        {!isEditing && (
          <ButtonSecondary
            size="large"
            width="50%"
            disabled={currentTab === StandardEditorTab.Overview}
            onClick={goToPreviousTab}
          >
            Back
          </ButtonSecondary>
        )}
      </ActionButtonsContainer>
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
