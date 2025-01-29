/**
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

import { useEffect, useState } from 'react';
import styled from 'styled-components';

import { ButtonPrimary, Flex, Text } from 'design';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { DocumentTerminal } from 'teleterm/ui/DocumentTerminal';
import * as types from 'teleterm/ui/services/workspacesService';
import { connectToDatabase } from 'teleterm/ui/services/workspacesService';

/**
 * DocumentGatewayCliClient creates a terminal session that targets the given gateway.
 *
 * It waits for the gateway to be created before starting the terminal session. This typically
 * happens only during the app restart. We assume that most of the time both the DocumentGateway and
 * DocumentGatewayCliClient tabs will be reopened together. In that case, DocumentGatewayCliClient
 * will wait for DocumentGateway to create the gateway first before attempting to start the client.
 *
 * However, if the user closes just the DocumentGateway tab and then restarts the app with just the
 * DocumentGatewayCliClient tab present, the gateway will never be created. In that case, the user
 * will be able to click "Open the connection" to manually open a new DocumentGateway tab.
 */
export const DocumentGatewayCliClient = (props: {
  visible: boolean;
  doc: types.DocumentGatewayCliClient;
}) => {
  const { clustersService } = useAppContext();
  clustersService.useState();

  const { doc, visible } = props;
  const [hasRenderedTerminal, setHasRenderedTerminal] = useState(false);

  const gateway = clustersService.findGatewayByConnectionParams({
    targetUri: doc.targetUri,
    targetUser: doc.targetUser,
  });

  // Once we render the terminal, we want to keep it visible. Otherwise removing the gateway would
  // mean that this document would immediately unmount DocumentTerminal and close the PTY.
  //
  // After the gateway is closed, the CLI client will not be able to interact with the gateway
  // target, but the user might still want to inspect the output.
  if (gateway || hasRenderedTerminal) {
    if (!hasRenderedTerminal) {
      setHasRenderedTerminal(true);
    }

    return <DocumentTerminal doc={doc} visible={visible} />;
  }

  return <WaitingForGateway doc={doc} visible={visible} />;
};

const TIMEOUT_SECONDS = 10;

const WaitingForGateway = (props: {
  doc: types.DocumentGatewayCliClient;
  visible: boolean;
}) => {
  const { doc, visible } = props;
  const ctx = useAppContext();
  const { documentsService } = useWorkspaceContext();
  // If we depended on doc.status for hasTimedOut instead of using a separate state, then on reopen
  // the doc would have status set to 'connected' on 'error' and it'd be updated from useEffect,
  // meaning that there would be a brief flash of old state.
  const [hasTimedOut, setHasTimedOut] = useState(false);

  useEffect(() => {
    // Update the doc state to make the progress bar show up in the tab bar.
    // Once DocumentTerminal is mounted, it is going to update the status to 'connected' or 'error'.
    documentsService.update(doc.uri, { status: 'connecting' });

    const timeoutId = setTimeout(() => {
      setHasTimedOut(true);
      documentsService.update(doc.uri, { status: 'error' });
    }, TIMEOUT_SECONDS * 1000);

    return () => {
      clearTimeout(timeoutId);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const openConnection = () => {
    connectToDatabase(
      ctx,
      {
        uri: doc.targetUri,
        name: doc.targetName,
        protocol: doc.targetProtocol,
        dbUser: doc.targetUser,
      },
      { origin: 'reopened_session' }
    );
  };

  return (
    <Document visible={visible} px={2}>
      <WaitingForGatewayContent
        doc={doc}
        hasTimedOut={hasTimedOut}
        openConnection={openConnection}
      />
    </Document>
  );
};

export const WaitingForGatewayContent = ({
  doc,
  hasTimedOut,
  openConnection,
}: {
  doc: types.DocumentGatewayCliClient;
  hasTimedOut: boolean;
  openConnection: () => void;
}) => (
  <Flex gap={4} flexDirection="column" mx="auto" alignItems="center" mt={100}>
    {hasTimedOut ? (
      <div>
        <StyledText>
          A connection to <strong>{doc.targetName}</strong> as{' '}
          <strong>{doc.targetUser}</strong> has not been opened up within{' '}
          {TIMEOUT_SECONDS} seconds.
        </StyledText>
        <StyledText>Please try to open the connection manually.</StyledText>
      </div>
    ) : (
      <StyledText>
        Waiting for a db connection to <strong>{doc.targetName}</strong> as{' '}
        <strong>{doc.targetUser}</strong> to be opened up.
      </StyledText>
    )}

    <ButtonPrimary onClick={openConnection}>Open the connection</ButtonPrimary>
  </Flex>
);

const StyledText = styled(Text).attrs({
  typography: 'body1',
  textAlign: 'center',
})``;
