/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState, useRef } from 'react';
import styled from 'styled-components';

import Popover from 'design/Popover';
import { MoreVert, OpenBox, Add } from 'design/Icon';
import { Box, Text, Flex } from 'design';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { TopBarButton } from 'teleterm/ui/TopBar/TopBarButton';
import { RootClusterUri } from 'teleterm/ui/uri';

import { useIdentity } from '../Identity/useIdentity';

import { NavigationItem } from './NavigationItem';

function getNavigationItems(
  documentsService: DocumentsService,
  clusterUri: RootClusterUri
): {
  title: string;
  Icon: JSX.Element;
  onNavigate: () => void;
}[] {
  return [
    {
      title: 'New Access Request',
      Icon: <Add fontSize={2} />,
      onNavigate: () => {
        const doc = documentsService.createAccessRequestDocument({
          clusterUri,
          state: 'creating',
          title: 'New Access Request',
        });
        documentsService.add(doc);
        documentsService.open(doc.uri);
      },
    },
    {
      title: 'Review Access Requests',
      Icon: <OpenBox fontSize={2} />,
      onNavigate: () => {
        const doc = documentsService.createAccessRequestDocument({
          clusterUri,
          state: 'browsing',
        });
        documentsService.add(doc);
        documentsService.open(doc.uri);
      },
    },
  ];
}

export function NavigationMenu() {
  const ctx = useAppContext();
  const documentsService =
    ctx.workspacesService.getActiveWorkspaceDocumentService();
  const { activeRootCluster } = useIdentity();

  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const selectorRef = useRef<HTMLButtonElement>();

  const shouldShowMenu = !!activeRootCluster?.features?.advancedAccessWorkflows;
  if (!shouldShowMenu) {
    return null;
  }

  return (
    <>
      <TopBarButton
        ref={selectorRef}
        isOpened={isPopoverOpened}
        title="Go To Access Requests"
        onClick={() => setIsPopoverOpened(true)}
      >
        <MoreVert fontSize={6} />
      </TopBarButton>
      <Popover
        open={isPopoverOpened}
        anchorEl={selectorRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        onClose={() => setIsPopoverOpened(false)}
        popoverCss={() => `max-width: min(560px, 90%)`}
      >
        <MenuContainer p={3}>
          <Box minWidth="280px">
            <Text fontWeight={700}>Go To</Text>
            <Flex flexDirection="column">
              {getNavigationItems(documentsService, activeRootCluster.uri).map(
                (item, index) => (
                  <NavigationItem
                    key={index}
                    item={item}
                    closeMenu={() => setIsPopoverOpened(false)}
                  />
                )
              )}
            </Flex>
          </Box>
        </MenuContainer>
      </Popover>
    </>
  );
}

const MenuContainer = styled(Box)`
  background: ${props => props.theme.colors.levels.surface};
`;
