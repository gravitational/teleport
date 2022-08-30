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

import { Flex, Text, ButtonPrimary } from 'design';

import { Danger } from 'design/Alert';

import Document from 'teleterm/ui/Document';

import * as types from 'teleterm/ui/services/workspacesService';

import { useReconnect, State } from './useReconnect';

export default function Container(props: DocumentReconnectProps) {
  const state = useReconnect(props.doc);
  return <DocumentReconnect visible={props.visible} {...state} />;
}

export function DocumentReconnect(props: State & { visible: boolean }) {
  return (
    <Document visible={props.visible} flexDirection="column" pl={2}>
      <Flex flexDirection="column" mx="auto" alignItems="center" mt={100}>
        {props.attempt.status === 'failed' && (
          <Danger mb={3}>{props.attempt.statusText}</Danger>
        )}
        <Text typography="h5" color="text.primary">
          This SSH connection is currently offline
        </Text>
        <ButtonPrimary mt={4} width="100px" onClick={props.reconnect}>
          Reconnect
        </ButtonPrimary>
      </Flex>
    </Document>
  );
}

type DocumentReconnectProps = {
  doc: types.DocumentTshNode;
  visible: boolean;
};
