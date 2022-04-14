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
import { Text, Flex, Box, ButtonPrimary, ButtonSecondary, Link } from 'design';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import Document from 'teleterm/ui/Document';
import * as Alerts from 'design/Alert';
import * as types from 'teleterm/ui/services/workspacesService';
import LinearProgress from 'teleterm/ui/components/LinearProgress';
import useDocumentGateway, { State } from './useDocumentGateway';

type Props = {
  visible: boolean;
  doc: types.DocumentGateway;
};

export default function Container(props: Props) {
  const { doc, visible } = props;
  const state = useDocumentGateway(doc);
  return (
    <Document visible={visible}>
      <DocumentGateway {...state} />
    </Document>
  );
}

export function DocumentGateway(props: State) {
  const { gateway, connected, connectAttempt, disconnect, reconnect } = props;

  if (!connected) {
    const statusDescription =
      connectAttempt.status === 'processing' ? 'being set up' : 'offline';

    return (
      <Flex flexDirection="column" mx="auto" alignItems="center" mt={100}>
        <Text
          typography="h5"
          color="text.primary"
          style={{ position: 'relative' }}
        >
          The database connection is {statusDescription}
          {connectAttempt.status === 'processing' && <LinearProgress />}
        </Text>
        {connectAttempt.status === 'error' && (
          <Alerts.Danger>{connectAttempt.statusText}</Alerts.Danger>
        )}
        <ButtonPrimary
          mt={4}
          width="100px"
          onClick={reconnect}
          disabled={connectAttempt.status === 'processing'}
        >
          Reconnect
        </ButtonPrimary>
      </Flex>
    );
  }

  return (
    <Box maxWidth="1024px" mx="auto" mt="4" px="5">
      <Flex justifyContent="space-between" mb="4">
        <Text typography="h3" color="text.secondary">
          Database Connection
        </Text>
        <ButtonSecondary size="small" onClick={disconnect}>
          Close Connection
        </ButtonSecondary>
      </Flex>
      <Text bold>Connect with CLI</Text>
      <TextSelectCopy
        bash={true}
        bg={'primary.dark'}
        mb={4}
        text={gateway.cliCommand}
      />
      <Text bold>Connect with GUI</Text>
      <Text>
        To connect with a GUI database client, see our{' '}
        <Link
          href="https://goteleport.com/docs/database-access/guides/gui-clients/"
          target="_blank"
        >
          documentation
        </Link>{' '}
        for instructions.
      </Text>
    </Box>
  );
}
