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

import { MutableRefObject, useMemo } from 'react';
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
import { DocumentGateway } from 'teleterm/ui/DocumentGateway';
import { DocumentGatewayApp } from 'teleterm/ui/DocumentGatewayApp';
import { DocumentGatewayCliClient } from 'teleterm/ui/DocumentGatewayCliClient';
import { DocumentGatewayKube } from 'teleterm/ui/DocumentGatewayKube';
import { DocumentTerminal } from 'teleterm/ui/DocumentTerminal';
import * as types from 'teleterm/ui/services/workspacesService';
import {
  DocumentsService,
  Workspace,
} from 'teleterm/ui/services/workspacesService';
import { isAppUri, isDatabaseUri, RootClusterUri } from 'teleterm/ui/uri';

import { KeyboardShortcutsPanel } from './KeyboardShortcutsPanel';
import { WorkspaceContextProvider } from './workspaceContext';

export function DocumentsRenderer(props: {
  topBarConnectMyComputerRef: MutableRefObject<HTMLDivElement>;
  topBarAccessRequestRef: MutableRefObject<HTMLDivElement>;
}) {
  const { workspacesService } = useAppContext();

  function renderDocuments(documentsService: DocumentsService) {
    return documentsService.getDocuments().map(doc => {
      const isActiveDoc = workspacesService.isDocumentActive(doc.uri);
      return <MemoizedDocument doc={doc} visible={isActiveDoc} key={doc.uri} />;
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
      // DELETE IN 15.0.0. See DocumentGatewayKube for more details.
      case 'doc.terminal_tsh_kube':
        return <DocumentTerminal doc={doc} visible={visible} />;
      case 'doc.access_requests':
        return <DocumentAccessRequests doc={doc} visible={visible} />;
      case 'doc.connect_my_computer':
        return <DocumentConnectMyComputer doc={doc} visible={visible} />;
      case 'doc.authorize_web_session':
        return <DocumentAuthorizeWebSession doc={doc} visible={visible} />;
      default:
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
