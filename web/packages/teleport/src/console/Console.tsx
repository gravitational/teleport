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
import AjaxPoller from 'teleport/components/AjaxPoller';
import { useConsoleContext } from './consoleContextProvider';
import * as stores from './stores/types';
import { colors } from './components/colors';
import Tabs from './components/Tabs';
import ActionBar from './components/ActionBar';
import DocumentSsh from './components/DocumentSsh';
import DocumentNodes from './components/DocumentNodes';
import DocumentBlank from './components/DocumentBlank';
import useRouting from './useRouting';

const POLL_INTERVAL = 5000; // every 5 sec

export default function Console() {
  const consoleCtx = useConsoleContext();
  const { clusterId, activeDocId } = useRouting(consoleCtx);
  const storeDocs = consoleCtx.storeDocs;
  const hasSshSessions = storeDocs.getSshDocuments().length > 0;

  function onTabClick(doc: stores.Document) {
    consoleCtx.gotoTab(doc);
  }

  function onTabClose(doc: stores.Document) {
    consoleCtx.closeTab(doc);
  }

  function onTabNew() {
    consoleCtx.gotoNodeTab(clusterId);
  }

  function onRefresh() {
    return consoleCtx.refreshParties();
  }

  function onLogout() {
    consoleCtx.logout();
  }

  const disableNewTab = storeDocs.getNodeDocuments().length > 0;
  const documents = storeDocs.getDocuments();
  const $docs = documents.map(doc => (
    <MemoizedDocument doc={doc} visible={doc.id === activeDocId} key={doc.id} />
  ));

  return (
    <StyledConsole>
      <Flex bg="primary.dark" height="38px">
        <Tabs
          flex="1"
          items={documents}
          onClose={onTabClose}
          onSelect={onTabClick}
          activeTab={activeDocId}
        />
        <ActionBar
          clusterId={clusterId}
          disableAddTab={disableNewTab}
          onNew={onTabNew}
          onLogout={onLogout}
        />
      </Flex>
      {$docs}
      {hasSshSessions && (
        <AjaxPoller time={POLL_INTERVAL} onFetch={onRefresh} />
      )}
    </StyledConsole>
  );
}

/**
 * Ensures that document is not getting re-rendered if it's invisible
 */
function MemoizedDocument(props: { doc: stores.Document; visible: boolean }) {
  const { doc, visible } = props;
  return React.useMemo(() => {
    switch (doc.kind) {
      case 'terminal':
        return <DocumentSsh doc={doc} visible={visible} />;
      case 'nodes':
        return <DocumentNodes doc={doc} visible={visible} />;
      default:
        return <DocumentBlank doc={doc} visible={visible} />;
    }
  }, [visible, doc]);
}

const StyledConsole = styled.div`
  background-color: ${colors.bgTerminal};
  bottom: 0;
  left: 0;
  position: absolute;
  right: 0;
  top: 0;
  display: flex;
  flex-direction: column;
`;
