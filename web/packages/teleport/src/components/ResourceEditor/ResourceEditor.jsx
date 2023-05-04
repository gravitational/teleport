/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import PropTypes from 'prop-types';
import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';
import {
  Box,
  ButtonOutlined,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  LabelInput,
  Text,
} from 'design';
import TextEditor from 'shared/components/TextEditor';
import * as Alerts from 'design/Alert';
import { useAttempt, useState } from 'shared/hooks';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

export default function ResourceEditor(props) {
  const {
    title,
    text,
    name,
    directions = null,
    docsURL = null,
    onClose,
    isNew,
    kind = '',
  } = props;

  const { attempt, attemptActions, content, isDirty, setContent } =
    useEditor(text);

  const roleResource = kind === 'role';

  const onSave = () => {
    attemptActions
      .do(() => props.onSave(content))
      .then(() => {
        if (roleResource) {
          userEventService.captureUserEvent({
            event: CaptureEvent.CreateNewRoleSaveClickEvent,
          });
        }
        onClose();
      });
  };

  const handleClose = () => {
    if (roleResource) {
      userEventService.captureUserEvent({
        event: CaptureEvent.CreateNewRoleCancelClickEvent,
      });
    }

    onClose();
  };

  const isSaveDisabled = attempt.isProcessing || (!isDirty && !isNew);
  const hasDirections = directions && docsURL;

  return (
    <Dialog open={true} dialogCss={dialogCss} onClose={onClose}>
      <Flex flex="1">
        <Flex flex="1" m={5} flexDirection="column">
          <DialogHeader>
            <DialogTitle typography="body1" bold>
              {title}
            </DialogTitle>
          </DialogHeader>
          {attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
          {!isNew && (
            <Text mb="2" typography="h4" color="text.contrast">
              {name}
            </Text>
          )}
          <LabelInput>Spec</LabelInput>
          <Flex flex="1">
            <TextEditor
              readOnly={false}
              data={[{ content, type: 'yaml' }]}
              onChange={setContent}
            />
          </Flex>
          <Box mt="5">
            <ButtonPrimary disabled={isSaveDisabled} onClick={onSave} mr="3">
              Save changes
            </ButtonPrimary>
            <ButtonSecondary
              disabled={attempt.isProcessing}
              onClick={handleClose}
            >
              CANCEL
            </ButtonSecondary>
          </Box>
        </Flex>
        {hasDirections && (
          <Flex
            flexDirection="column"
            justifyContent="space-between"
            height="100%"
            width="300px"
            p={5}
            bg="levels.surface"
          >
            <Box>
              <DialogTitle typography="body1" bold>
                {' '}
                SETUP INSTRUCTIONS{' '}
              </DialogTitle>
              <Text typography="body1" mt={3}>
                {directions}
              </Text>
            </Box>
            <ButtonOutlined
              size="medium"
              as="a"
              href={docsURL}
              target="_blank"
              width="100%"
              rel="noreferrer"
              onClick={() => {
                if (roleResource) {
                  userEventService.captureUserEvent({
                    event:
                      CaptureEvent.CreateNewRoleViewDocumentationClickEvent,
                  });
                }
              }}
            >
              VIEW DOCUMENTATION
            </ButtonOutlined>
          </Flex>
        )}
      </Flex>
    </Dialog>
  );
}

ResourceEditor.propTypes = {
  name: PropTypes.string,
  text: PropTypes.string,
  title: PropTypes.string,
  docsURL: PropTypes.string,
  data: PropTypes.string,
  onSave: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  isNew: PropTypes.bool.isRequired,
  directions: PropTypes.element,
  kind: PropTypes.string,
};

const dialogCss = () => `
  height: 80%;
  width: calc(100% - 20%);
  max-width: 1400px;
  padding: 0;
`;

function useEditor(initial) {
  const [attempt, attemptActions] = useAttempt();
  const [state, setState] = useState({
    isDirty: false,
    content: initial,
  });

  function setContent(content) {
    setState({
      isDirty: initial !== content,
      content,
    });
  }

  return {
    ...state,
    attempt,
    attemptActions,
    setContent,
  };
}
