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
import { MoreVert, OpenBox, Add, Config } from 'design/Icon';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { TopBarButton } from 'teleterm/ui/TopBar/TopBarButton';
import { IAppContext } from 'teleterm/ui/types';
import { Cluster } from 'teleterm/services/tshd/types';

import { NavigationItem } from './NavigationItem';

function useNavigationItems(): (
  | {
      type?: 'normal';
      title: string;
      Icon: JSX.Element;
      onNavigate: () => void;
    }
  | { type: 'separator' }
)[] {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();

  const documentsService =
    ctx.workspacesService.getActiveWorkspaceDocumentService();
  const activeRootCluster = getActiveRootCluster(ctx);
  const accessRequestsAllowed =
    !!activeRootCluster?.features?.advancedAccessWorkflows;

  return [
    {
      title: 'Open Config File',
      Icon: <Config fontSize={2} />,
      onNavigate: () => {
        ctx.mainProcessClient.openConfigFile();
      },
    },
    { type: 'separator' as const },
    accessRequestsAllowed && {
      title: 'New Access Request',
      Icon: <Add fontSize={2} />,
      onNavigate: () => {
        const doc = documentsService.createAccessRequestDocument({
          clusterUri: activeRootCluster.uri,
          state: 'creating',
          title: 'New Access Request',
        });
        documentsService.add(doc);
        documentsService.open(doc.uri);
      },
    },
    accessRequestsAllowed && {
      title: 'Review Access Requests',
      Icon: <OpenBox fontSize={2} />,
      onNavigate: () => {
        const doc = documentsService.createAccessRequestDocument({
          clusterUri: activeRootCluster.uri,
          state: 'browsing',
        });
        documentsService.add(doc);
        documentsService.open(doc.uri);
      },
    },
  ].filter(Boolean);
}

function getActiveRootCluster(ctx: IAppContext): Cluster | undefined {
  const clusterUri = ctx.workspacesService.getRootClusterUri();
  if (!clusterUri) {
    return;
  }
  return ctx.clustersService.findCluster(clusterUri);
}

export function NavigationMenu() {
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const selectorRef = useRef<HTMLButtonElement>();

  const items = useNavigationItems().map((item, index, items) => {
    if (item.type === 'separator') {
      return index !== items.length - 1 && <Separator key={index} />; // if not the last element
    }
    return (
      <NavigationItem
        key={index}
        item={item}
        closeMenu={() => setIsPopoverOpened(false)}
      />
    );
  });

  return (
    <>
      <TopBarButton
        ref={selectorRef}
        isOpened={isPopoverOpened}
        title="See more"
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
        <Menu>{items}</Menu>
      </Popover>
    </>
  );
}

const Menu = styled.menu`
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  min-width: 280px;
  background: ${props => props.theme.colors.primary.light};
`;

const Separator = styled.div`
  background: ${props => props.theme.colors.primary.lighter};
  height: 1px;
`;
