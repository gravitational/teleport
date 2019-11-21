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
import { Flex } from 'design';
import useConsoleContext, {
  useStoreDialogs,
  SessionStateEnum,
} from 'teleport/console/useConsoleContext';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import Terminal from './../Terminal';
import Home from './../Home';
import FileTransferDialog from './../FileTransfer';

export default function Document({ visible, doc }) {
  const { type, id } = doc;
  const Doc = type === 'terminal' ? DocumentTerminal : DocumentHome;
  return <Doc doc={doc} visible={visible} key={id} />;
}

export function DocumentHome({ visible }) {
  const consoleContext = useConsoleContext();
  function onNew(login, serverId) {
    const { url } = consoleContext.addTerminalTab({ login, serverId });
    history.push(url);
  }

  return (
    <StyledDocument visible={visible}>
      <Home visible={visible} onNew={onNew} clusterId={cfg.clusterName} />
    </StyledDocument>
  );
}

export function DocumentTerminal({ doc, visible }) {
  const { sid, clusterId, serverId, login, id } = doc;
  const dialogs = useStoreDialogs();
  const consoleContext = useConsoleContext();

  function onCloseScp() {
    dialogs.close(id);
  }

  function onConnect(session) {
    const { serverId, hostname, login, sid } = session;
    const { url } = consoleContext.updateConnectedTerminalTab(id, {
      serverId,
      hostname,
      login,
      sid,
    });

    history.replace(url);
  }

  function onDisconnect() {
    consoleContext.storeDocs.updateItem(id, {
      status: SessionStateEnum.DISCONNECTED,
    });
  }

  const { isDownloadOpen, isUploadOpen } = dialogs.getState(id);

  return (
    <StyledDocument visible={visible}>
      <Terminal
        sid={sid}
        tabId={id}
        clusterId={clusterId}
        serverId={serverId}
        login={login}
        onConnect={onConnect}
        onDisconnect={onDisconnect}
        onSessionEnd={onDisconnect}
      />
      <FileTransferDialog
        tabId={id}
        clusterId={clusterId}
        serverId={serverId}
        login={login}
        isDownloadOpen={isDownloadOpen}
        isUploadOpen={isUploadOpen}
        onClose={onCloseScp}
      />
    </StyledDocument>
  );
}

function StyledDocument({ children, visible }) {
  return (
    <Flex
      flex="1"
      style={{
        overflow: 'auto',
        display: visible ? 'flex' : 'none',
        position: 'relative',
      }}
    >
      {children}
    </Flex>
  );
}
