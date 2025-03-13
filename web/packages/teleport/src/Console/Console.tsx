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

import { Fragment, useEffect, useMemo } from 'react';
import styled from 'styled-components';

import { Box, Flex, Indicator } from 'design';
import { Danger } from 'design/Alert';
import useAttempt from 'shared/hooks/useAttemptNext';

import AjaxPoller from 'teleport/components/AjaxPoller';
import { ParticipantMode } from 'teleport/services/session';

import ActionBar from './ActionBar';
import { useConsoleContext, useStoreDocs } from './consoleContextProvider';
import DocumentBlank from './DocumentBlank';
import { DocumentDb } from './DocumentDb';
import DocumentKubeExec from './DocumentKubeExec';
import DocumentNodes from './DocumentNodes';
import DocumentSsh from './DocumentSsh';
import * as stores from './stores/types';
import Tabs from './Tabs';
import useKeyboardNav from './useKeyboardNav';
import useOnExitConfirmation from './useOnExitConfirmation';
import usePageTitle from './usePageTitle';
import useTabRouting from './useTabRouting';

const POLL_INTERVAL = 1000; // every 1 sec

export default function Console() {
  const consoleCtx = useConsoleContext();
  const { verifyAndConfirm } = useOnExitConfirmation(consoleCtx);
  const { clusterId, activeDocId } = useTabRouting(consoleCtx);

  const storeDocs = consoleCtx.storeDocs;
  const documents = storeDocs.getDocuments();
  const activeDoc = documents.find(d => d.id === activeDocId);
  const hasSshSessions = storeDocs.getSshDocuments().length > 0;
  const { attempt, run } = useAttempt();

  useEffect(() => {
    run(() => consoleCtx.initStoreUser());
  }, []);

  useKeyboardNav(consoleCtx);
  useStoreDocs(consoleCtx);
  usePageTitle(activeDoc);

  function onTabClick(doc: stores.Document) {
    consoleCtx.gotoTab(doc);
  }

  function onTabClose(doc: stores.Document) {
    if (verifyAndConfirm(doc)) {
      consoleCtx.closeTab(doc);
    }
  }

  function onTabNew() {
    consoleCtx.gotoNodeTab(clusterId);
  }

  function onRefresh() {
    return consoleCtx.refreshParties();
  }

  const disableNewTab =
    storeDocs.getNodeDocuments().length > 0 ||
    storeDocs.getDbDocuments().length > 0;
  const $docs = documents.map(doc => (
    <Fragment key={doc.id}>
      {doc.id === activeDocId && (
        <MemoizedDocument
          doc={doc}
          visible={doc.id === activeDocId}
          key={doc.id}
          mode={doc.kind === 'terminal' ? doc.mode : null}
        />
      )}
    </Fragment>
  ));

  return (
    <StyledConsole>
      {attempt.status === 'failed' && (
        <Danger>{`Error: ${attempt.statusText} (Try refreshing the page)`}</Danger>
      )}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <>
          <Flex bg="levels.surface" height="32px">
            <Tabs
              flex="1"
              items={documents}
              onClose={onTabClose}
              onSelect={onTabClick}
              activeTab={activeDocId}
              clusterId={clusterId}
              disableNew={disableNewTab}
              onNew={onTabNew}
            />
            <ActionBar
              latencyIndicator={
                activeDoc?.kind === 'terminal'
                  ? {
                      isVisible: true,
                      latency: activeDoc.latency,
                    }
                  : { isVisible: false }
              }
            />
          </Flex>
          {$docs}
          {hasSshSessions && (
            <AjaxPoller time={POLL_INTERVAL} onFetch={onRefresh} />
          )}
        </>
      )}
    </StyledConsole>
  );
}

/**
 * Ensures that document is not getting re-rendered if it's invisible
 */
function MemoizedDocument(props: {
  doc: stores.Document;
  visible: boolean;
  mode: ParticipantMode;
}) {
  const { doc, visible } = props;
  return useMemo(() => {
    switch (doc.kind) {
      case 'terminal':
        return <DocumentSsh doc={doc} visible={visible} mode={props.mode} />;
      case 'nodes':
        return <DocumentNodes doc={doc} visible={visible} />;
      case 'kubeExec':
        return <DocumentKubeExec doc={doc} visible={visible} />;
      case 'db':
        return <DocumentDb doc={doc} visible={visible} />;
      default:
        return <DocumentBlank doc={doc} visible={visible} />;
    }
  }, [visible, doc]);
}

const StyledConsole = styled.div`
  background-color: ${props => props.theme.colors.levels.sunken};
  bottom: 0;
  left: 0;
  position: absolute;
  right: 0;
  top: 0;
  display: flex;
  flex-direction: column;
  max-height: 100%;
`;
