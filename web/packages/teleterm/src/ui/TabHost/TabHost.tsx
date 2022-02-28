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
import styled from 'styled-components';
import { Flex } from 'design';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as types from 'teleterm/ui/services/docs/types';
import { Tabs } from 'teleterm/ui/Tabs';
import Document from 'teleterm/ui/Document';
import DocumentHome from 'teleterm/ui/DocumentHome';
import DocumentGateway from 'teleterm/ui/DocumentGateway';
import DocumentTerminal from 'teleterm/ui/DocumentTerminal';
import DocumentCluster from 'teleterm/ui/DocumentCluster';
import { useTabShortcuts } from './useTabShortcuts';

export function TabHost() {
  const ctx = useAppContext();
  const { docsService } = ctx;
  const documents = docsService.getDocuments();
  const docActive = docsService.getActive();

  // enable keyboard shortcuts
  useTabShortcuts();

  // subscribe
  docsService.useState();

  function handleTabClick(doc: types.Document) {
    docsService.open(doc.uri);
  }

  function handleTabClose(doc: types.Document) {
    docsService.close(doc.uri);
  }

  function handleTabMoved(oldIndex: number, newIndex: number) {
    docsService.swapPosition(oldIndex, newIndex);
  }

  function handleTabNew() {
    docsService.openNewTerminal();
  }

  function handleTabContextMenu(doc: types.Document) {
    ctx.mainProcessClient.openTabContextMenu({
      documentKind: doc.kind,
      onClose: () => {
        docsService.close(doc.uri);
      },
      onCloseOthers: () => {
        docsService.closeOthers(doc.uri);
      },
      onCloseToRight: () => {
        docsService.closeToRight(doc.uri);
      },
      onDuplicatePty: () => {
        docsService.duplicatePtyAndActivate(doc.uri);
      },
    });
  }

  const $docs = documents.map(doc => {
    const isActiveDoc = doc === docActive;
    return <MemoizedDocument doc={doc} visible={isActiveDoc} key={doc.uri} />;
  });

  return (
    <StyledTabHost>
      <Flex bg="terminalDark" height="32px">
        <Tabs
          flex="1"
          items={documents.filter(d => d.kind !== 'doc.home')}
          onClose={handleTabClose}
          onSelect={handleTabClick}
          onContextMenu={handleTabContextMenu}
          activeTab={docActive.uri}
          onMoved={handleTabMoved}
          disableNew={false}
          onNew={handleTabNew}
        />
      </Flex>
      {$docs}
    </StyledTabHost>
  );
}

function MemoizedDocument(props: { doc: types.Document; visible: boolean }) {
  const { doc, visible } = props;
  return React.useMemo(() => {
    switch (doc.kind) {
      case 'doc.home':
        return <DocumentHome doc={doc} visible={visible} />;
      case 'doc.cluster':
        return <DocumentCluster doc={doc} visible={visible} />;
      case 'doc.gateway':
        return <DocumentGateway doc={doc} visible={visible} />;
      case 'doc.terminal_shell':
      case 'doc.terminal_tsh_node':
      case 'doc.terminal_tsh_kube':
        return <DocumentTerminal doc={doc} visible={visible} />;
      default:
        return (
          <Document visible={visible}>
            Document kind "{doc.kind}" is not supported
          </Document>
        );
    }
  }, [visible, doc]);
}

const StyledTabHost = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  right: 0;
`;