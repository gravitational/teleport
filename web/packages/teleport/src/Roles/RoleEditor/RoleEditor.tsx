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

import { useCallback, useEffect, useId, useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, ButtonSecondary, ButtonWarning, Flex, P2 } from 'design';
import { Danger, Info } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Validation, { Validator } from 'shared/components/Validation';
import { Attempt, useAsync } from 'shared/hooks/useAsync';

import { CatchError } from 'teleport/components/CatchError';
import cfg from 'teleport/config';
import { Role, RoleWithYaml } from 'teleport/services/resources';
import { storageService } from 'teleport/services/storageService';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';

import { EditorHeader } from './EditorHeader';
import { EditorTab } from './EditorTabs';
import { unableToUpdatePreviewMessage } from './Shared';
import { StandardEditor } from './StandardEditor/StandardEditor';
import {
  roleEditorModelToRole,
  roleToRoleEditorModel,
} from './StandardEditor/standardmodel';
import {
  ActionType,
  useStandardModel,
} from './StandardEditor/useStandardModel';
import { YamlEditor } from './YamlEditor';
import { YamlEditorModel } from './yamlmodel';

export type RoleEditorProps = {
  /**
   * Describes an original role and its YAML representation. `null` or
   * `undefined` if the user is creating a new role.
   */
  originalRole?: RoleWithYaml;
  /**
   * An attempt to show the role diff component. Used to show an error state.
   */
  roleDiffAttempt?: Attempt<unknown>;
  onCancel?(): void;
  onSave?(r: Partial<RoleWithYaml>): Promise<void>;
  onRoleUpdate?(r: Role): void;
  demoMode?: boolean;
};

/**
 * Renders a role editor that consists of a standard (structural) editor and a
 * raw YAML editor as a fallback for cases where the role contains elements
 * unsupported by the standard editor.
 */
export const RoleEditor = ({
  originalRole,
  roleDiffAttempt,
  onCancel,
  onSave,
  onRoleUpdate,
  demoMode,
}: RoleEditorProps) => {
  const roleTesterEnabled =
    (cfg.isPolicyEnabled && storageService.getAccessGraphRoleTesterEnabled()) ||
    demoMode;
  const idPrefix = useId();
  // These IDs are needed to connect accessibility attributes between the
  // standard/YAML tab switcher and the switched panels.
  const standardEditorId = `${idPrefix}-standard`;
  const yamlEditorId = `${idPrefix}-yaml`;

  const [standardModel, dispatch] = useStandardModel(originalRole?.object);

  useEffect(() => {
    const { roleModel, validationResult } = standardModel;
    if (roleTesterEnabled && roleModel && validationResult?.isValid) {
      onRoleUpdate?.(roleEditorModelToRole(roleModel));
    }
  }, [standardModel, onRoleUpdate, roleTesterEnabled, demoMode]);

  const [yamlModel, setYamlModel] = useState<YamlEditorModel>({
    content: originalRole?.yaml ?? '',
    isDirty: !originalRole, // New role is dirty by default.
  });

  const isDirty = (): boolean => {
    switch (selectedEditorTab) {
      case EditorTab.Standard:
        return standardModel.isDirty;
      case EditorTab.Yaml:
        return yamlModel.isDirty;
      default:
        selectedEditorTab satisfies never;
    }
  };

  // Defaults to yaml editor if the role could not be parsed.
  const [selectedEditorTab, setSelectedEditorTab] = useState<EditorTab>(() => {
    const { roleModel } = standardModel;
    return roleModel === undefined || roleModel.requiresReset
      ? EditorTab.Yaml
      : EditorTab.Standard;
  });

  // Converts YAML representation to a standard editor model.
  const [parseAttempt, parseYaml] = useAsync(async () => {
    try {
      const parsedRole = await yamlModelToRole(yamlModel);
      return roleToRoleEditorModel(parsedRole, originalRole?.object);
    } catch (err) {
      throw new Error('Unable to load role into the standard editor', {
        cause: err,
      });
    }
  });

  // The standard editor will automatically preview the changes based on state updates
  // but the yaml editor needs to be told when to update (the preview button)
  const [yamlPreviewAttempt, handleYamlPreview] = useAsync(
    useCallback(async () => {
      if (!onRoleUpdate) {
        return;
      }
      try {
        const newRole = await yamlModelToRole(yamlModel);
        onRoleUpdate(newRole);
      } catch (err) {
        throw new Error(unableToUpdatePreviewMessage, { cause: err });
      }
    }, [onRoleUpdate, yamlModel])
  );

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

  const [confirmingExit, setConfirmingExit] = useState(false);

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
    if (
      standardModel.roleModel !== undefined &&
      !standardModel.roleModel?.requiresReset &&
      !validator.validate()
    )
      return;
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

        dispatch({
          type: ActionType.SetModel,
          payload: roleModel,
        });
        break;
      }
      case EditorTab.Yaml: {
        if (standardModel.roleModel?.requiresReset) {
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

  function confirmExit() {
    if (isDirty()) {
      setConfirmingExit(true);
    } else {
      handleExit();
    }
  }

  function closeExitConfirmation() {
    setConfirmingExit(false);
  }

  function handleExit() {
    userEventService.captureUserEvent({
      event: CaptureEvent.CreateNewRoleCancelClickEvent,
    });
    onCancel();
  }

  return (
    <>
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
                onClose={confirmExit}
              />
              <AttemptAlert attempt={saveAttempt} />
              <AttemptAlert attempt={parseAttempt} />
              <AttemptAlert attempt={yamlifyAttempt} />
              <AttemptAlert attempt={yamlPreviewAttempt} />
              <AttemptAlert attempt={roleDiffAttempt} />
            </Box>
            {selectedEditorTab === EditorTab.Standard && (
              <Flex flexDirection="column" flex="1" id={standardEditorId}>
                <CatchError fallbackFn={StandardEditorRenderingError}>
                  <StandardEditor
                    originalRole={originalRole}
                    onSave={object => handleSave({ object })}
                    standardEditorModel={standardModel}
                    isProcessing={isProcessing}
                    dispatch={dispatch}
                  />
                </CatchError>
              </Flex>
            )}
            {/* Hiding instead of unmounting the info alert allows us to keep
                the dismissed state throughout the lifetime of the role editor
                without keeping this state in the editor model. */}
            <ShowHide hidden={selectedEditorTab !== EditorTab.Yaml}>
              <Info dismissible mx={3} mb={3} alignItems="flex-start">
                Not all YAML edits can be represented in the standard editor.
                You may have to revert changes in the YAML if you return to
                using the standard editor.
              </Info>
            </ShowHide>
            {selectedEditorTab === EditorTab.Yaml && (
              <Flex flexDirection="column" flex="1" id={yamlEditorId}>
                <YamlEditor
                  yamlEditorModel={yamlModel}
                  onChange={setYamlModel}
                  onSave={async yaml => void (await handleSave({ yaml }))}
                  isProcessing={isProcessing}
                  originalRole={originalRole}
                  onPreview={roleTesterEnabled ? handleYamlPreview : undefined}
                />
              </Flex>
            )}
          </Flex>
        )}
      </Validation>

      <Dialog open={confirmingExit} onClose={closeExitConfirmation}>
        <DialogHeader mb={4}>
          <DialogTitle>Are you sure you want to close the editor?</DialogTitle>
        </DialogHeader>
        <DialogContent mb={3}>
          <P2>
            The role you are editing contains unsaved changes. If you close the
            editor, these changes will be lost.
          </P2>
        </DialogContent>
        <Flex gap={3}>
          <ButtonWarning block size="large" onClick={handleExit}>
            Discard Changes and Close
          </ButtonWarning>
          <ButtonSecondary
            block
            size="large"
            autoFocus
            onClick={closeExitConfirmation}
          >
            Keep Editing
          </ButtonSecondary>
        </Flex>
      </Dialog>
    </>
  );
};

const yamlModelToRole = ({ content }: YamlEditorModel) =>
  yamlService.parse<Role>(YamlSupportedResourceKind.Role, {
    yaml: content,
  });

/** A custom  fallback component for {@link CatchError}. */
const StandardEditorRenderingError = ({ error }: { error: Error }) => (
  <Alert mx={3} details={error.message}>
    Unable to render the standard editor. Please switch to the YAML editor
    above.
  </Alert>
);

/** Renders an alert if attempt is defined and has failed. */
const AttemptAlert = ({ attempt }: { attempt?: Attempt<unknown> }) => {
  if (attempt?.status !== 'error') return;
  return attempt.error ? (
    <ErrorAlert error={attempt.error} />
  ) : (
    <Danger mt={3} dismissible>
      {attempt.statusText}
    </Danger>
  );
};

/** Renders an alert if there is an error. */
const ErrorAlert = ({ error }: { error: Error }) =>
  error && (
    <Danger mt={3} dismissible details={error.cause?.toString()}>
      {error.message}
    </Danger>
  );

const ShowHide = styled.div<{ hidden: boolean }>`
  display: ${props => (props.hidden ? 'none' : '')};
`;
