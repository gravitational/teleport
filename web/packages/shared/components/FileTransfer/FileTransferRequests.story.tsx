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

import ConsoleContext from 'teleport/Console/consoleContext';
import ConsoleContextProvider from 'teleport/Console/consoleContextProvider';
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
