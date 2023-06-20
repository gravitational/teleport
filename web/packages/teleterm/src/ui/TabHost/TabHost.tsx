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
import * as types from 'teleterm/ui/services/workspacesService/documentsService/types';
import { Tabs } from 'teleterm/ui/Tabs';
import { DocumentsRenderer } from 'teleterm/ui/Documents/DocumentsRenderer';
import { IAppContext } from 'teleterm/ui/types';
import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';

import { useTabShortcuts } from './useTabShortcuts';
import { useNewTabOpener } from './useNewTabOpener';
import { ClusterConnectPanel } from './ClusterConnectPanel/ClusterConnectPanel';

export function TabHostContainer() {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  const isRootClusterSelected = !!ctx.workspacesService.getRootClusterUri();

  if (isRootClusterSelected) {
    return <TabHost ctx={ctx} />;
  }
  return <ClusterConnectPanel />;
}

export function TabHost({ ctx }: { ctx: IAppContext }) {
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
      documentKind: doc.kind,
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
      <DocumentsRenderer />
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
