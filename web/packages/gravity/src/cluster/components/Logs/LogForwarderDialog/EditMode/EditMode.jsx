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
import { DialogHeader, DialogTitle } from 'design/Dialog';
import {
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Text,
  Box,
  LabelInput,
} from 'design';
import TextEditor from 'shared/components/TextEditor';
import * as Alerts from 'design/Alert';
import { useAttempt, useState } from 'shared/hooks';
import example from './../template';

export default function EditMode({ item, isNew, onSave, onCancel, ...styles }) {
  const initialContent = isNew ? example : item.content;
  const [attempt, attemptActions] = useAttempt();
  const [content, setContent] = useState(initialContent);
  const [isDirty, setDirty] = useState(false);

  function onChange(value) {
    setContent(value);
    setDirty(value !== initialContent && !!value);
  }

  function onSaveClick() {
    attemptActions.do(() => onSave(content));
  }

  const isSaveDisabled = attempt.isProcessing || (!isDirty && !isNew);
  const title = isNew ? 'Create New Log Forwarder' : 'Edit Log Forwarder';
  const { height, width } = styles;

  return (
    <Flex height={height} width={width}>
      <Flex flex="1" p="5" flexDirection="column">
        <DialogHeader>
          <DialogTitle typography="body1" bold>
            {title}
          </DialogTitle>
        </DialogHeader>
        {attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
        {!isNew && (
          <Text mb="2" typography="h4" color="primary.contrastText">
            {item.displayName}
          </Text>
        )}
        <LabelInput>Spec</LabelInput>
        <Flex flex="1">
          <TextEditor
            readOnly={false}
            data={[{ content, type: 'yaml' }]}
            onChange={onChange}
          />
        </Flex>
        <Box mt="5">
          <ButtonPrimary disabled={isSaveDisabled} onClick={onSaveClick} mr="3">
            Save changes
          </ButtonPrimary>
          <ButtonSecondary disabled={attempt.isProcessing} onClick={onCancel}>
            CANCEL
          </ButtonSecondary>
        </Box>
      </Flex>
      <Flex
        flexDirection="column"
        ml="auto"
        justifyContent="space-between"
        height="100%"
        width="280px"
        p="5"
        bg="primary.light"
      >
        <Box>
          <DialogTitle typography="body1" bold mb="3">
            About Log forwarding
          </DialogTitle>
          <Text typography="paragraph">
            A Gravity cluster aggregates the logs from all running containers.
            Use log forwarders to ship cluster logs to a remote log collector
            such as a rsyslog server.
          </Text>
        </Box>
        <ButtonSecondary
          as="a"
          width="100%"
          href="https://gravitational.com/gravity/docs/cluster/#configuring-log-forwarders"
          target="_blank"
        >
          VIEW DOCUMENTATION
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}

EditMode.defaultProps = {
  height: '500px',
  width: '800px',
};
