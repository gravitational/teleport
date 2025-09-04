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

import { MutableRefObject, ReactNode, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import styled from 'styled-components';

import { Text } from 'design';

import {
  AccessRequestsContextProvider,
  AccessRequestsMenu,
} from 'teleterm/ui/AccessRequests';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  ConnectMyComputerContextProvider,
  ConnectMyComputerNavigationMenu,
  DocumentConnectMyComputer,
} from 'teleterm/ui/ConnectMyComputer';
import Document from 'teleterm/ui/Document';
import { DocumentAccessRequests } from 'teleterm/ui/DocumentAccessRequests';
import { DocumentAuthorizeWebSession } from 'teleterm/ui/DocumentAuthorizeWebSession';
import DocumentCluster from 'teleterm/ui/DocumentCluster';
import { DocumentDesktopSession } from 'teleterm/ui/DocumentDesktopSession';
import { DocumentGateway } from 'teleterm/ui/DocumentGateway';
import { DocumentGatewayApp } from 'teleterm/ui/DocumentGatewayApp';
import { DocumentGatewayCliClient } from 'teleterm/ui/DocumentGatewayCliClient';
import { DocumentGatewayKube } from 'teleterm/ui/DocumentGatewayKube';
import { DocumentTerminal } from 'teleterm/ui/DocumentTerminal';
import { useIsInBackgroundMode } from 'teleterm/ui/hooks/useIsInBackgroundMode';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import * as types from 'teleterm/ui/services/workspacesService';
import {
  DocumentsService,
  Workspace,
} from 'teleterm/ui/services/workspacesService';
import { isAppUri, isDatabaseUri, RootClusterUri } from 'teleterm/ui/uri';
import { DocumentVnetDiagReport } from 'teleterm/ui/Vnet/DocumentVnetDiagReport';
import { DocumentVnetInfo } from 'teleterm/ui/Vnet/DocumentVnetInfo';

import { KeyboardShortcutsPanel } from './KeyboardShortcutsPanel';
import { WorkspaceContextProvider } from './workspaceContext';

export function DocumentsRenderer(props: {
  topBarConnectMyComputerRef: MutableRefObject<HTMLDivElement>;
  topBarAccessRequestRef: MutableRefObject<HTMLDivElement>;
}) {
  const { workspacesService } = useAppContext();
  const isAnyDialogOpen = useStoreSelector('modalsService', state => {
    return !!state.regular || state.important.length > 0;
  });
  const isInBackgroundMode = useIsInBackgroundMode();

  function renderDocuments(documentsService: DocumentsService) {
    return documentsService.getDocuments().map(doc => {
      const isActiveDoc = workspacesService.isDocumentActive(doc.uri);
      const document = <MemoizedDocument doc={doc} visible={isActiveDoc} />;
      const { kind } = doc;
      switch (kind) {
        case 'doc.authorize_web_session':
        case 'doc.access_requests':
        case 'doc.blank':
        case 'doc.cluster':
        case 'doc.connect_my_computer':
        case 'doc.vnet_diag_report':
        case 'doc.vnet_info':
        case 'doc.gateway':
          // Mount the document when it becomes visible.
          return (
            <MountOnVisible key={doc.uri} visible={isActiveDoc}>
              {document}
            </MountOnVisible>
          );
        // Documents that should be terminated when the window is hidden:
        case 'doc.desktop_session':
        case 'doc.gateway_cli_client':
        case 'doc.gateway_kube':
        case 'doc.terminal_shell':
        case 'doc.terminal_tsh_node':
          const isConnected =
            kind === 'doc.terminal_shell' || kind === 'doc.terminal_tsh_node'
              ? true
              : doc.status === 'connected';
          return (
            <ForegroundSession
              isInBackgroundMode={isInBackgroundMode}
              isDocumentActive={isActiveDoc}
              isDocumentConnected={isConnected}
              isAnyDialogOpen={isAnyDialogOpen}
              key={doc.uri}
            >
              {document}
            </ForegroundSession>
          );
        default:
          kind satisfies never;
      }
    });
  }

  const workspaces = useMemo(
    () =>
      Object.entries(workspacesService.getWorkspaces()).map(
        ([clusterUri, workspace]: [RootClusterUri, Workspace]) => ({
          rootClusterUri: clusterUri,
          localClusterUri: workspace.localClusterUri,
          documentsService:
            workspacesService.getWorkspaceDocumentService(clusterUri),
          accessRequestsService:
            workspacesService.getWorkspaceAccessRequestsService(clusterUri),
        })
      ),
    [workspacesService.getWorkspaces()]
  );

  return (
    <>
      {workspaces.map(workspace => (
        <DocumentsContainer
          isVisible={
            workspace.rootClusterUri === workspacesService.getRootClusterUri()
          }
          key={workspace.rootClusterUri}
        >
          <WorkspaceContextProvider value={workspace}>
            <ConnectMyComputerContextProvider
              rootClusterUri={workspace.rootClusterUri}
            >
              <AccessRequestsContextProvider
                rootClusterUri={workspace.rootClusterUri}
              >
                {workspace.documentsService.getDocuments().length ? (
                  renderDocuments(workspace.documentsService)
                ) : (
                  <KeyboardShortcutsPanel />
                )}
                {workspace.rootClusterUri ===
                  workspacesService.getRootClusterUri() && (
                  <>
                    {props.topBarConnectMyComputerRef.current &&
                      createPortal(
                        <ConnectMyComputerNavigationMenu />,
                        props.topBarConnectMyComputerRef.current
                      )}
                    {props.topBarAccessRequestRef.current &&
                      createPortal(
                        <AccessRequestsMenu />,
                        props.topBarAccessRequestRef.current
                      )}
                  </>
                )}
              </AccessRequestsContextProvider>
            </ConnectMyComputerContextProvider>
          </WorkspaceContextProvider>
        </DocumentsContainer>
      ))}
    </>
  );
}

const DocumentsContainer = styled.div<{ isVisible?: boolean }>`
  display: ${props => (props.isVisible ? 'contents' : 'none')};
`;

function MemoizedDocument(props: { doc: types.Document; visible: boolean }) {
  const { doc, visible } = props;

  return useMemo(() => {
    switch (doc.kind) {
      case 'doc.cluster':
        return <DocumentCluster doc={doc} visible={visible} />;
      case 'doc.gateway': {
        //TODO(gzdunek): Reorganize the code related to gateways.
        // We should have a parent DocumentGateway component that
        // would render DocumentGatewayDatabase and DocumentGatewayApp.
        if (isDatabaseUri(doc.targetUri)) {
          return <DocumentGateway doc={doc} visible={visible} />;
        }
        if (isAppUri(doc.targetUri)) {
          return <DocumentGatewayApp doc={doc} visible={visible} />;
        }
        return (
          <Document visible={visible}>
            <Text m="auto" mt={10} textAlign="center">
              Cannot create a gateway for the target "{doc.targetUri}".
              <br />
              Only database, kube, and app targets are supported.
            </Text>
          </Document>
        );
      }
      case 'doc.gateway_cli_client':
        return <DocumentGatewayCliClient doc={doc} visible={visible} />;
      case 'doc.gateway_kube':
        return <DocumentGatewayKube doc={doc} visible={visible} />;
      case 'doc.terminal_shell':
      case 'doc.terminal_tsh_node':
        return <DocumentTerminal doc={doc} visible={visible} />;
      case 'doc.access_requests':
        return <DocumentAccessRequests doc={doc} visible={visible} />;
      case 'doc.connect_my_computer':
        return <DocumentConnectMyComputer doc={doc} visible={visible} />;
      case 'doc.authorize_web_session':
        return <DocumentAuthorizeWebSession doc={doc} visible={visible} />;
      case 'doc.vnet_diag_report':
        return <DocumentVnetDiagReport doc={doc} visible={visible} />;
      case 'doc.vnet_info':
        return <DocumentVnetInfo doc={doc} visible={visible} />;
      case 'doc.desktop_session':
        return <DocumentDesktopSession doc={doc} visible={visible} />;
      default:
        doc satisfies types.DocumentBlank;
        return (
          <Document visible={visible}>
            <Text m="auto" mt={10} textAlign="center">
              Document kind "{doc.kind}" is not supported.
            </Text>
          </Document>
        );
    }
  }, [visible, doc]);
}

/**
 * Wrapper for sessions that require user interaction.
 *
 * The component:
 * 1. Defers rendering the document until it's active.
 *    This prevents spamming the user with MFA prompts when restoring a session
 *    on launch or reopening a previously hidden window.
 * 2. Unmounts the document when it is connected but the window is hidden.
 *    This terminates the related connection, which will be restored when
 *    the window is visible again.
 *    Mounting the document is paused until dialogs are closed.
 *    Since showing the window can be triggered by a tshd event,
 *    it’s important to handle that event first (typically via a dialog) before
 *    displaying any MFA dialogs that may be required for resource access.
 */
function ForegroundSession({
  isDocumentActive,
  isDocumentConnected,
  isAnyDialogOpen,
  isInBackgroundMode,
  children,
}: {
  isDocumentActive: boolean;
  isDocumentConnected: boolean;
  isInBackgroundMode: boolean;
  isAnyDialogOpen: boolean;
  children: ReactNode;
}) {
  if (isInBackgroundMode && isDocumentConnected) {
    return;
  }

  return (
    <MountOnVisible
      visible={!isInBackgroundMode && isDocumentActive && !isAnyDialogOpen}
    >
      {children}
    </MountOnVisible>
  );
}

/** Defers mounting the children until they are visible. */
function MountOnVisible({
  visible,
  children,
}: {
  visible: boolean;
  children: ReactNode;
}) {
  const [showChildren, setShowChildren] = useState(visible);

  if (!showChildren && visible) {
    setShowChildren(true);
  }

  return showChildren ? children : undefined;
}
