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

import { useId, useState } from 'react';
import { Box, Flex } from 'design';
import { useValidation } from 'shared/components/Validation';
import * as Icon from 'design/Icon';
import styled from 'styled-components';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { SlideTabs } from 'design/SlideTabs';
import { Role, RoleWithYaml } from 'teleport/services/resources';

import { EditorSaveCancelButton } from '../Shared';

import {
  roleEditorModelToRole,
  hasModifiedFields,
  RoleEditorModel,
  StandardEditorModel,
  AccessSpecKind,
  AccessSpec,
  newAccessSpec,
  RuleModel,
  OptionsModel,
} from './standardmodel';
import { validateRoleEditorModel } from './validation';
import { RequiresResetToStandard } from './RequiresResetToStandard';
import { MetadataSection } from './MetadataSection';
import { AccessSpecSection, specSections } from './Resources';
import { AccessRules } from './AccessRules';
import { Options } from './Options';

export type StandardEditorProps = {
  originalRole: RoleWithYaml;
  standardEditorModel: StandardEditorModel;
  isProcessing?: boolean;
  onSave?(r: Role): void;
  onCancel?(): void;
  onChange?(s: StandardEditorModel): void;
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
  onChange,
}: StandardEditorProps) => {
  const isEditing = !!originalRole;
  const { roleModel } = standardEditorModel;
  const validation = validateRoleEditorModel(roleModel);

  /** All spec kinds except those that are already in the role. */
  const allowedSpecKinds = allAccessSpecKinds.filter(k =>
    roleModel.accessSpecs.every(as => as.kind !== k)
  );

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

  function addAccessSpec(kind: AccessSpecKind) {
    handleChange({
      ...standardEditorModel.roleModel,
      accessSpecs: [
        ...standardEditorModel.roleModel.accessSpecs,
        newAccessSpec(kind),
      ],
    });
  }

  function removeAccessSpec(kind: AccessSpecKind) {
    handleChange({
      ...standardEditorModel.roleModel,
      accessSpecs: standardEditorModel.roleModel.accessSpecs.filter(
        s => s.kind !== kind
      ),
    });
  }

  function setAccessSpec(value: AccessSpec) {
    handleChange({
      ...standardEditorModel.roleModel,
      accessSpecs: standardEditorModel.roleModel.accessSpecs.map(original =>
        original.kind === value.kind ? value : original
      ),
    });
  }

  function setRules(rules: RuleModel[]) {
    handleChange({
      ...standardEditorModel.roleModel,
      rules,
    });
  }

  function setOptions(options: OptionsModel) {
    handleChange({
      ...standardEditorModel,
      options,
    });
  }

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
                  validator.state.validating && !validation.metadata.valid
                    ? validationErrorTabStatus
                    : undefined,
              },
              {
                key: StandardEditorTab.Resources,
                title: 'Resources',
                controls: resourcesTabId,
                status:
                  validator.state.validating &&
                  validation.accessSpecs.some(s => !s.valid)
                    ? validationErrorTabStatus
                    : undefined,
              },
              {
                key: StandardEditorTab.AccessRules,
                title: 'Access Rules',
                controls: accessRulesTabId,
                status:
                  validator.state.validating &&
                  validation.rules.some(s => !s.valid)
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
              validation={validation.metadata}
              onChange={metadata => handleChange({ ...roleModel, metadata })}
            />
          </Box>
          <Box
            id={resourcesTabId}
            style={{
              display: currentTab === StandardEditorTab.Resources ? '' : 'none',
            }}
          >
            <Flex flexDirection="column" gap={3} my={2}>
              {roleModel.accessSpecs.map((spec, i) => {
                const validationResult = validation.accessSpecs[i];
                return (
                  <AccessSpecSection
                    key={spec.kind}
                    value={spec}
                    isProcessing={isProcessing}
                    validation={validationResult}
                    onChange={value => setAccessSpec(value)}
                    onRemove={() => removeAccessSpec(spec.kind)}
                  />
                );
              })}
              <Box>
                <MenuButton
                  menuProps={{
                    transformOrigin: {
                      vertical: 'bottom',
                      horizontal: 'right',
                    },
                    anchorOrigin: {
                      vertical: 'top',
                      horizontal: 'right',
                    },
                  }}
                  buttonText={
                    <>
                      <Icon.Plus size="small" mr={2} />
                      Add New Specifications
                    </>
                  }
                  buttonProps={{
                    size: 'medium',
                    fill: 'filled',
                    disabled: isProcessing || allowedSpecKinds.length === 0,
                  }}
                >
                  {allowedSpecKinds.map(kind => (
                    <MenuItem key={kind} onClick={() => addAccessSpec(kind)}>
                      {specSections[kind].title}
                    </MenuItem>
                  ))}
                </MenuButton>
              </Box>
            </Flex>
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
              onChange={setRules}
              validation={validation.rules}
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
        disabled={
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

/**
 * All access spec kinds, in order of appearance in the resource kind dropdown.
 */
const allAccessSpecKinds: AccessSpecKind[] = [
  'kube_cluster',
  'node',
  'app',
  'db',
  'windows_desktop',
];

export const EditorWrapper = styled(Flex)<{ mute?: boolean }>`
  flex-direction: column;
  flex: 1;
  opacity: ${p => (p.mute ? 0.4 : 1)};
  pointer-events: ${p => (p.mute ? 'none' : '')};
`;
