/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useRef } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import {
  InfoGuideButton,
  InfoGuidePanelProvider,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import { LongGuideContent } from 'shared/components/SlidingSidePanel/InfoGuide/storyHelper';

import {
  makeRootCluster,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import { ConnectMyComputerContextProvider } from 'teleterm/ui/ConnectMyComputer';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeDocumentCluster } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';
import { VnetContextProvider } from 'teleterm/ui/Vnet';

import { StatusBar } from '../StatusBar';
import { StyledTabs } from '../Tabs';
import { TopBar } from '../TopBar';
import { InfoGuideSidePanel as MainComponent } from './InfoGuideSidePanel';
import { ResourcesContextProvider } from './resourcesContext';

export default {
  title: 'Teleterm/DocumentCluster/InfoGuideSidePanel',
};

const rootClusterDoc = makeDocumentCluster({
  clusterUri: rootClusterUri,
  uri: '/docs/123',
});

export function InfoGuideSidePanel() {
  const topBarConnectMyComputerRef = useRef<HTMLDivElement>(null);
  const topBarAccessRequestRef = useRef<HTMLDivElement>(null);

  const appContext = new MockAppContext();
  const cluster = makeRootCluster({
    uri: rootClusterDoc.clusterUri,
  });
  appContext.addRootClusterWithDoc(cluster, rootClusterDoc);

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <MockWorkspaceContextProvider>
            <ResourcesContextProvider>
              <ConnectMyComputerContextProvider rootClusterUri={rootClusterUri}>
                <Wrapper>
                  <InfoGuidePanelProvider>
                    <Flex flexDirection="column" height="100%">
                      <Flex flex="1" flexDirection="column">
                        <TopBar
                          connectMyComputerRef={topBarConnectMyComputerRef}
                          accessRequestRef={topBarAccessRequestRef}
                        />
                        <StyledTabs width="100%" pl={2}>
                          Dummy tab just for height placement for the guide
                          info.
                        </StyledTabs>
                        <Example />
                      </Flex>
                      <StatusBar onAssumedRolesClick={() => null} />
                    </Flex>
                  </InfoGuidePanelProvider>
                </Wrapper>
              </ConnectMyComputerContextProvider>
            </ResourcesContextProvider>
          </MockWorkspaceContextProvider>
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

const Wrapper = styled.div`
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
`;

function Example() {
  return (
    <>
      <Box mt={3}>
        <InfoGuideButton config={{ guide: <LongGuideContent /> }}>
          <Box ml={3}>Click the info button</Box>
        </InfoGuideButton>
      </Box>
      <MainComponent />
    </>
  );
}
