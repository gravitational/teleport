import React from 'react';

import ConsoleContextProvider from 'teleport/Console/consoleContextProvider';
import ConsoleContext from 'teleport/Console/consoleContext';
import { FileTransferRequest } from 'teleport/Console/DocumentSsh/useFileTransfer';

import { FileTransferRequests } from './';

export default {
  title: 'Shared/FileTransfer',
};

export const Requests = () => {
  const conCtx = new ConsoleContext();
  conCtx.storeUser.setState({ username: 'bob' });
  return (
    <ConsoleContextProvider value={conCtx}>
      <FileTransferRequests
        requests={requests}
        onApprove={() => {}}
        onDeny={() => {}}
      />
    </ConsoleContextProvider>
  );
};

const requests: FileTransferRequest[] = [
  {
    sid: 'dummy-sid',
    requestID: 'dummy-request-id',
    requester: 'alice',
    approvers: [],
    location: '/etc/teleport.yaml',
    download: true,
  },
  {
    sid: 'dummy-sid',
    requestID: 'dummy-request-id',
    requester: 'john',
    approvers: ['bob'],
    location: '/home/alice/.ssh/config',
    download: true,
  },
];
