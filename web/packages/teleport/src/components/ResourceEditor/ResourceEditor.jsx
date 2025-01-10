/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import PropTypes from 'prop-types';
import { useState } from 'react';

import {
  Box,
  ButtonBorder,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H3,
  LabelInput,
  Text,
} from 'design';
import * as Alerts from 'design/Alert';
import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';
import TextEditor from 'shared/components/TextEditor';
import { useAttempt } from 'shared/hooks';

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
            <DialogTitle>{title}</DialogTitle>
          </DialogHeader>
          {attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
          {!isNew && (
            <Text mb="2" typography="body1">
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
              Cancel
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
              <H3>Setup Instructions</H3>
              <Text mt={3}>{directions}</Text>
            </Box>
            <ButtonBorder
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
              View Documentation
            </ButtonBorder>
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
