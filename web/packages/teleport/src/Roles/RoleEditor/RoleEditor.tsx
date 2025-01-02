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

import { Alert, Box, Flex } from 'design';
import Validation, { Validator } from 'shared/components/Validation';
import { useAsync } from 'shared/hooks/useAsync';
import { Role, RoleWithYaml } from 'teleport/services/resources';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';

import { EditorHeader } from './EditorHeader';
import { EditorTab } from './EditorTabs';
import { StandardEditor } from './StandardEditor/StandardEditor';
import {
  newRole,
  roleEditorModelToRole,
  roleToRoleEditorModel,
  StandardEditorModel,
} from './StandardEditor/standardmodel';
import { YamlEditor } from './YamlEditor';
import { YamlEditorModel } from './yamlmodel';

export type RoleEditorProps = {
  /**
   * Describes an original role and its YAML representation. `null` or
   * `undefined` if the user is creating a new role.
   */
  originalRole?: RoleWithYaml;
  onCancel?(): void;
  onSave?(r: Partial<RoleWithYaml>): Promise<void>;
};

/**
 * Renders a role editor that consists of a standard (structural) editor and a
 * raw YAML editor as a fallback for cases where the role contains elements
 * unsupported by the standard editor.
 */
export const RoleEditor = ({
  originalRole,
  onCancel,
  onSave,
}: RoleEditorProps) => {
  const idPrefix = useId();
  // These IDs are needed to connect accessibility attributes between the
  // standard/YAML tab switcher and the switched panels.
  const standardEditorId = `${idPrefix}-standard`;
  const yamlEditorId = `${idPrefix}-yaml`;

  const [standardModel, setStandardModel] = useState<StandardEditorModel>(
    () => {
      const role = originalRole?.object ?? newRole();
      const roleModel = roleToRoleEditorModel(role, role);
      return {
        roleModel,
        isDirty: !originalRole, // New role is dirty by default.
      };
    }
  );

  const [yamlModel, setYamlModel] = useState<YamlEditorModel>({
    content: originalRole?.yaml ?? '',
    isDirty: !originalRole, // New role is dirty by default.
  });

  // Defaults to yaml editor if the role could not be parsed.
  const [selectedEditorTab, setSelectedEditorTab] = useState<EditorTab>(() =>
    standardModel.roleModel.requiresReset ? EditorTab.Yaml : EditorTab.Standard
  );

  // Converts YAML representation to a standard editor model.
  const [parseAttempt, parseYaml] = useAsync(async () => {
    const parsedRole = await yamlService.parse<Role>(
      YamlSupportedResourceKind.Role,
      {
        yaml: yamlModel.content,
      }
    );
    return roleToRoleEditorModel(parsedRole, originalRole?.object);
  });

  // Converts standard editor model to a YAML representation.
  const [yamlifyAttempt, yamlifyRole] = useAsync(
    async () =>
      await yamlService.stringify(YamlSupportedResourceKind.Role, {
        resource: roleEditorModelToRole(standardModel.roleModel),
      })
  );

  const [saveAttempt, handleSave] = useAsync(
    async (r: Partial<RoleWithYaml>) => {
      await onSave?.(r);
      userEventService.captureUserEvent({
        event: CaptureEvent.CreateNewRoleSaveClickEvent,
      });
    }
  );

  const isProcessing =
    parseAttempt.status === 'processing' ||
    yamlifyAttempt.status === 'processing' ||
    saveAttempt.status === 'processing';

  async function onTabChange(activeIndex: EditorTab, validator: Validator) {
    // The code below is not idempotent, so we need to protect ourselves from
    // an accidental model replacement.
    if (activeIndex === selectedEditorTab) return;

    // Validate the model on tab switch, because the server-side yamlification
    // requires model to be valid. However, if it's OK, we reset the validator.
    // We don't want it to be validating at this point, since the user didn't
    // attempt to submit the form.
    if (!standardModel.roleModel.requiresReset && !validator.validate()) return;
    validator.reset();

    switch (activeIndex) {
      case EditorTab.Standard: {
        if (!yamlModel.content) {
          //  nothing to parse.
          return;
        }
        const [roleModel, err] = await parseYaml();
        // Abort if there's an error. Don't switch the tab or set the model.
        if (err) return;

        setStandardModel({
          roleModel,
          isDirty: yamlModel.isDirty,
        });
        break;
      }
      case EditorTab.Yaml: {
        if (standardModel.roleModel.requiresReset) {
          break;
        }
        const [content, err] = await yamlifyRole();
        // Abort if there's an error. Don't switch the tab or set the model.
        if (err) return;

        setYamlModel({
          content,
          isDirty: originalRole?.yaml != content,
        });
        break;
      }
      default:
        activeIndex satisfies never;
    }

    setSelectedEditorTab(activeIndex);
  }

  function handleCancel() {
    userEventService.captureUserEvent({
      event: CaptureEvent.CreateNewRoleCancelClickEvent,
    });
    onCancel();
  }

  return (
    <Validation>
      {({ validator }) => (
        <Flex flexDirection="column" flex="1">
          <Box mt={3} mx={3}>
            <EditorHeader
              role={originalRole?.object}
              selectedEditorTab={selectedEditorTab}
              onEditorTabChange={index => onTabChange(index, validator)}
              isProcessing={isProcessing}
              standardEditorId={standardEditorId}
              yamlEditorId={yamlEditorId}
              onClose={onCancel}
            />
            {saveAttempt.status === 'error' && (
              <Alert mt={3} dismissible>
                {saveAttempt.statusText}
              </Alert>
            )}
            {parseAttempt.status === 'error' && (
              <Alert mt={3} dismissible>
                {parseAttempt.statusText}
              </Alert>
            )}
            {yamlifyAttempt.status === 'error' && (
              <Alert mt={3} dismissible>
                {yamlifyAttempt.statusText}
              </Alert>
            )}
          </Box>
          {selectedEditorTab === EditorTab.Standard && (
            <Flex flexDirection="column" flex="1" id={standardEditorId}>
              <StandardEditor
                originalRole={originalRole}
                onSave={object => handleSave({ object })}
                onCancel={handleCancel}
                standardEditorModel={standardModel}
                isProcessing={isProcessing}
                onChange={setStandardModel}
              />
            </Flex>
          )}
          {selectedEditorTab === EditorTab.Yaml && (
            <Flex flexDirection="column" flex="1" id={yamlEditorId}>
              <YamlEditor
                yamlEditorModel={yamlModel}
                onChange={setYamlModel}
                onSave={async yaml => void (await handleSave({ yaml }))}
                isProcessing={isProcessing}
                onCancel={handleCancel}
                originalRole={originalRole}
              />
            </Flex>
          )}
        </Flex>
      )}
    </Validation>
  );
};
