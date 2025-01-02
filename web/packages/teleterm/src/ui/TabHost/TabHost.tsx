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

import React, { useCallback } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';

import { Shell } from 'teleterm/mainProcess/shell';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { DocumentsRenderer } from 'teleterm/ui/Documents/DocumentsRenderer';
import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';
import { useWorkspaceServiceState } from 'teleterm/ui/services/workspacesService';
import * as types from 'teleterm/ui/services/workspacesService/documentsService/types';
import { canDocChangeShell } from 'teleterm/ui/services/workspacesService/documentsService/types';
import { Tabs } from 'teleterm/ui/Tabs';
import { IAppContext } from 'teleterm/ui/types';

import { useStoreSelector } from '../hooks/useStoreSelector';
import { ClusterConnectPanel } from './ClusterConnectPanel/ClusterConnectPanel';
import { useNewTabOpener } from './useNewTabOpener';
import { useTabShortcuts } from './useTabShortcuts';

export function TabHostContainer(props: {
  topBarContainerRef: React.MutableRefObject<HTMLDivElement>;
}) {
  const ctx = useAppContext();
  const isRootClusterSelected = useStoreSelector(
    'workspacesService',
    useCallback(state => !!state.rootClusterUri, [])
  );

  if (isRootClusterSelected) {
    return <TabHost ctx={ctx} topBarContainerRef={props.topBarContainerRef} />;
  }
  return <ClusterConnectPanel />;
}

export function TabHost({
  ctx,
  topBarContainerRef,
}: {
  ctx: IAppContext;
  topBarContainerRef: React.MutableRefObject<HTMLDivElement>;
}) {
  useWorkspaceServiceState();
  const documentsService =
    ctx.workspacesService.getActiveWorkspaceDocumentService();
  const activeDocument = documentsService?.getActive();

  // TODO(gzdunek): make workspace refactor - it'd be helpful to have a single object that fully represents a workspace
  const { openClusterTab } = useNewTabOpener({
    documentsService,
    localClusterUri:
      ctx.workspacesService.getActiveWorkspace()?.localClusterUri,
  });
  const { getLabelWithAccelerator } = useKeyboardShortcutFormatters();

  useTabShortcuts({
    documentsService,
    localClusterUri:
      ctx.workspacesService.getActiveWorkspace()?.localClusterUri,
  });

  function handleTabClick(doc: types.Document) {
    documentsService.open(doc.uri);
  }

  function handleTabClose(doc: types.Document) {
    documentsService.close(doc.uri);
  }

  function handleTabMoved(oldIndex: number, newIndex: number) {
    documentsService.swapPosition(oldIndex, newIndex);
  }

  function handleTabContextMenu(doc: types.Document) {
    ctx.mainProcessClient.openTabContextMenu({
      document: doc,
      onClose: () => {
        documentsService.close(doc.uri);
      },
      onCloseOthers: () => {
        documentsService.closeOthers(doc.uri);
      },
      onCloseToRight: () => {
        documentsService.closeToRight(doc.uri);
      },
      onDuplicatePty: () => {
        documentsService.duplicatePtyAndActivate(doc.uri);
      },
      onReopenPtyInShell(shell: Shell) {
        if (canDocChangeShell(doc)) {
          documentsService.reopenPtyInShell(doc, shell);
        }
      },
    });
  }

  function getActiveWorkspaceDocuments() {
    return ctx.workspacesService
      .getActiveWorkspaceDocumentService()
      .getDocuments();
  }

  return (
    <StyledTabHost>
      <Flex height="32px">
        <Tabs
          flex="1"
          items={getActiveWorkspaceDocuments()}
          onClose={handleTabClose}
          onSelect={handleTabClick}
          onContextMenu={handleTabContextMenu}
          activeTab={activeDocument?.uri}
          onMoved={handleTabMoved}
          onNew={openClusterTab}
          newTabTooltip={getLabelWithAccelerator('New Tab', 'newTab')}
          closeTabTooltip={getLabelWithAccelerator('Close', 'closeTab')}
        />
      </Flex>
      <DocumentsRenderer topBarContainerRef={topBarContainerRef} />
    </StyledTabHost>
  );
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
