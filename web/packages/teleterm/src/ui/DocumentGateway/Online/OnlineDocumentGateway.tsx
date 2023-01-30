import React, { useEffect } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { Gateway } from 'teleterm/services/tshd/types';
import { OnlineDocumentContainer } from 'teleterm/ui/DocumentGateway/Online/OnlineDocumentContainer';
import { GUIInstructions } from 'teleterm/ui/DocumentGateway/Online/GUIInstructions';
import { CLIForm } from 'teleterm/ui/DocumentGateway/Online/CLIForm';
import { Header } from 'teleterm/ui/DocumentGateway/Online/Header';
import { CLIInstructions } from 'teleterm/ui/DocumentGateway/Online/CLIInstructions';

interface OnlineDocumentGatewayProps {
  doc: types.DocumentGateway;
  gateway: Gateway;
}

export function OnlineDocumentGateway(props: OnlineDocumentGatewayProps) {
  const ctx = useAppContext();

  const { documentsService } = useWorkspaceContext();

  const [disconnectAttempt, disconnect] = useAsync(async () => {
    await ctx.clustersService.removeGateway(props.doc.gatewayUri);
  });

  useEffect(() => {
    if (disconnectAttempt.status === 'success') {
      documentsService.close(props.doc.uri);
    }
  }, [disconnectAttempt.status, props.doc]);

  return (
    <OnlineDocumentContainer>
      <Header onClose={disconnect} />

      <CLIInstructions>
        <CLIForm doc={props.doc} gateway={props.gateway} />
      </CLIInstructions>

      <GUIInstructions gateway={props.gateway} />
    </OnlineDocumentContainer>
  );
}
