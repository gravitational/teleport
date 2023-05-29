/*
Copyright 2023 Gravitational, Inc.

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

import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import { Flex, Text, ButtonPrimary } from 'design';
import { useAsync } from 'shared/hooks/useAsync';

import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { DocumentTerminal } from 'teleterm/ui/DocumentTerminal';
import { connectToKube } from 'teleterm/ui/services/workspacesService';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { routing } from 'teleterm/ui/uri';

export const DocumentGatewayKube = (props: {
  visible: boolean;
  doc: types.DocumentGatewayKube;
}) => {
  const { clustersService } = useAppContext();
  clustersService.useState();

  const { doc, visible } = props;
  const [hasRenderedTerminal, setHasRenderedTerminal] = useState(false);

  // TODO support user, groups, namespace
  const gateway = clustersService.findGatewayByConnectionParams(
    doc.targetUri,
    ''
  );

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
  doc: types.DocumentGatewayKube;
  visible: boolean;
}) => {
  const { doc, visible } = props;
  const ctx = useAppContext();
  const { documentsService } = useWorkspaceContext();
  // If we depended on doc.status for hasTimedOut instead of using a separate state, then on reopen
  // the doc would have status set to 'connected' on 'error' and it'd be updated from useEffect,
  // meaning that there would be a brief flash of old state.
  const [hasTimedOut, setHasTimedOut] = useState(false);
  const [connectAttempt, createGateway] = useAsync(async () => {
    const gw = await retryWithRelogin(ctx, doc.targetUri, () =>
      // TODO support user, groups, namespace
      ctx.clustersService.createGateway({
        targetUri: doc.targetUri,
        user: '',
      })
    );

    documentsService.update(doc.uri, {
      gatewayUri: gw.uri,
      port: gw.localPort,
    });
  });
  const { params } = routing.parseKubeUri(doc.targetUri);

  useEffect(() => {
    // Update the doc state to make the progress bar show up in the tab bar.
    // Once DocumentTerminal is mounted, it is going to update the status to 'connected' or 'error'.
    documentsService.update(doc.uri, { status: 'connecting' });

    if (connectAttempt.status === '') {
      createGateway();
    }

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
    connectToKube(
      ctx,
      {
        uri: doc.targetUri,
      },
      { origin: 'reopened_session' }
    );
  };

  return (
    <Document visible={visible} px={2}>
      <WaitingForGatewayContent
        kubeId={params.kubeId}
        hasTimedOut={hasTimedOut}
        openConnection={openConnection}
      />
    </Document>
  );
};

export const WaitingForGatewayContent = ({
  kubeId,
  hasTimedOut,
  openConnection,
}: {
  kubeId: string;
  hasTimedOut: boolean;
  openConnection: () => void;
}) => (
  <Flex gap={4} flexDirection="column" mx="auto" alignItems="center" mt={100}>
    {hasTimedOut ? (
      <div>
        <StyledText>
          A connection to <strong>{kubeId}</strong> has not been opened up
          within {TIMEOUT_SECONDS} seconds.
        </StyledText>
        <StyledText>Please try to open the connection manually.</StyledText>
      </div>
    ) : (
      <StyledText>
        Waiting for a kube connection to <strong>{kubeId}</strong> to be opened
        up.
      </StyledText>
    )}

    <ButtonPrimary onClick={openConnection}>Open the connection</ButtonPrimary>
  </Flex>
);

const StyledText = styled(Text).attrs({
  typography: 'h5',
  textAlign: 'center',
})``;
