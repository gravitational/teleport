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
import { useRouteMatch, useParams, useLocation } from 'react-router';
import styled from 'styled-components';
import cfg, { UrlSshParams } from 'teleport/config';
import { Flex } from 'design';
import AjaxPoller from 'teleport/components/AjaxPoller';
import { useConsoleContext, useStoreDocs } from './consoleContextProvider';
import * as stores from './stores/types';
import { colors } from './components/colors';
import Tabs from './components/Tabs';
import ActionBar from './components/ActionBar';
import DocumentSsh from './components/DocumentSsh';
import DocumentNodes from './components/DocumentNodes';
import DocumentBlank from './components/DocumentBlank';

const POLL_INTERVAL = 5000; // every 5 sec

export default function Console() {
  const consoleCtx = useConsoleContext();
  const { pathname } = useLocation();
  const { clusterId } = useParams<{ clusterId: string }>();
  const sshRouteMatch = useRouteMatch<UrlSshParams>(cfg.routes.consoleConnect);
  const nodesRouteMatch = useRouteMatch(cfg.routes.consoleNodes);
  const joinSshRouteMatch = useRouteMatch<UrlSshParams>(
    cfg.routes.consoleSession
  );

  // find the document which matches current URL
  const storeDocs = useStoreDocs();
  const hasSshSessions = storeDocs.getSshDocuments().length > 0;
  const activeDocId = consoleCtx.getActiveDocId(pathname);

  React.useEffect(() => {
    if (activeDocId !== -1) {
      return;
    }

    // when no document matches current URL that means we need to
    // create one base on URL parameters.
    if (sshRouteMatch) {
      consoleCtx.addSshDocument(sshRouteMatch.params);
    } else if (joinSshRouteMatch) {
      consoleCtx.addSshDocument(joinSshRouteMatch.params);
    } else if (nodesRouteMatch) {
      consoleCtx.addNodeDocument(clusterId);
    }
  }, [pathname]);

  function onSelectTab(doc: stores.Document) {
    consoleCtx.navigateTo(doc);
  }

  function onCloseTab(doc: stores.Document) {
    const next = consoleCtx.closeDocument(doc.id);
    consoleCtx.navigateTo(next);
  }

  function onNewTab() {
    const doc = consoleCtx.addNodeDocument(clusterId);
    consoleCtx.navigateTo(doc);
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
          onClose={onCloseTab}
          onSelect={onSelectTab}
          activeTab={activeDocId}
        />
        <ActionBar
          clusterId={clusterId}
          disableAddTab={disableNewTab}
          onNew={onNewTab}
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
  const $doc = React.useMemo(() => renderDocument(doc, visible), [
    visible,
    doc,
  ]);
  return $doc;
}

function renderDocument(doc: stores.Document, visible: boolean) {
  switch (doc.kind) {
    case 'terminal':
      return <DocumentSsh doc={doc} visible={visible} />;
    case 'nodes':
      return <DocumentNodes doc={doc} visible={visible} />;
    default:
      return <DocumentBlank doc={doc} visible={visible} />;
  }
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
