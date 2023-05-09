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

import { Flex, Text, Popover } from 'design';
import * as icons from 'design/Icon';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { TopBarButton } from 'teleterm/ui/TopBar/TopBarButton';
import { IAppContext } from 'teleterm/ui/types';
import { Cluster } from 'teleterm/services/tshd/types';
import { KeyboardShortcutAction } from 'teleterm/services/config';
import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { useNewTabOpener } from 'teleterm/ui/TabHost';

type MenuItem = {
  title: string;
  isVisible: boolean;
  Icon: React.ComponentType<{ fontSize: number }>;
  onNavigate: () => void;
  prependSeparator?: boolean;
  keyboardShortcutAction?: KeyboardShortcutAction;
};

function useMenuItems(): MenuItem[] {
  const ctx = useAppContext();
  const { workspacesService, mainProcessClient, notificationsService } = ctx;
  workspacesService.useState();
  ctx.clustersService.useState();
  const documentsService =
    workspacesService.getActiveWorkspaceDocumentService();
  const activeRootCluster = getActiveRootCluster(ctx);
  const { openTerminalTab } = useNewTabOpener({
    documentsService,
    localClusterUri: workspacesService.getActiveWorkspace()?.localClusterUri,
  });

  const areAccessRequestsSupported =
    !!activeRootCluster?.features?.advancedAccessWorkflows;

  const { platform } = mainProcessClient.getRuntimeSettings();
  const isDarwin = platform === 'darwin';

  const menuItems: MenuItem[] = [
    {
      title: 'Open new terminal',
      isVisible: true,
      Icon: icons.Terminal,
      keyboardShortcutAction: 'newTerminalTab',
      onNavigate: openTerminalTab,
    },
    {
      title: 'Open config file',
      isVisible: true,
      Icon: icons.Config,
      onNavigate: async () => {
        const path = await mainProcessClient.openConfigFile();
        notificationsService.notifyInfo(`Opened the config file at ${path}.`);
      },
    },
    {
      title: 'Install tsh in PATH',
      isVisible: isDarwin,
      Icon: icons.Link,
      onNavigate: () => {
        ctx.commandLauncher.executeCommand('tsh-install', undefined);
      },
    },
    {
      title: 'Remove tsh from PATH',
      isVisible: isDarwin,
      Icon: icons.Unlink,
      onNavigate: () => {
        ctx.commandLauncher.executeCommand('tsh-uninstall', undefined);
      },
    },
    {
      title: 'New access request',
      isVisible: areAccessRequestsSupported,
      prependSeparator: true,
      Icon: icons.Add,
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
    {
      title: 'Review access requests',
      isVisible: areAccessRequestsSupported,
      Icon: icons.OpenBox,
      onNavigate: () => {
        const doc = documentsService.createAccessRequestDocument({
          clusterUri: activeRootCluster.uri,
          state: 'browsing',
        });
        documentsService.add(doc);
        documentsService.open(doc.uri);
      },
    },
  ];

  return menuItems.filter(i => i.isVisible);
}

function getActiveRootCluster(ctx: IAppContext): Cluster | undefined {
  const clusterUri = ctx.workspacesService.getRootClusterUri();
  if (!clusterUri) {
    return;
  }
  return ctx.clustersService.findCluster(clusterUri);
}

export function AdditionalActions() {
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const selectorRef = useRef<HTMLButtonElement>();

  const items = useMenuItems().map(item => {
    return (
      <React.Fragment key={item.title}>
        {item.prependSeparator && <Separator />}
        <MenuItem item={item} closeMenu={() => setIsPopoverOpened(false)} />
      </React.Fragment>
    );
  });

  return (
    <>
      <TopBarButton
        ref={selectorRef}
        isOpened={isPopoverOpened}
        title="Additional Actions"
        onClick={() => setIsPopoverOpened(true)}
      >
        <icons.MoreVert fontSize={6} />
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
  background: ${props => props.theme.colors.levels.elevated};
`;

const Separator = styled.div`
  background: ${props => props.theme.colors.spotBackground[1]};
  height: 1px;
`;

function MenuItem({
  item,
  closeMenu,
}: {
  item: MenuItem;
  closeMenu: () => void;
}) {
  const { getAccelerator } = useKeyboardShortcutFormatters();
  const handleClick = () => {
    item.onNavigate();
    closeMenu();
  };

  return (
    <StyledListItem as="button" type="button" onClick={handleClick}>
      <item.Icon fontSize={2} />
      <Flex
        gap={2}
        flex="1"
        alignItems="baseline"
        justifyContent="space-between"
      >
        <Text>{item.title}</Text>

        {item.keyboardShortcutAction && (
          <Text
            fontSize={1}
            css={`
              border-radius: 4px;
              width: fit-content;
              padding: ${props => props.theme.space[1]}px
                ${props => props.theme.space[1]}px;
            `}
            bg="levels.surface"
          >
            {getAccelerator(item.keyboardShortcutAction)}
          </Text>
        )}
      </Flex>
    </StyledListItem>
  );
}

const StyledListItem = styled(ListItem)`
  height: 38px;
  gap: ${props => props.theme.space[3]}px;
  padding: 0 ${props => props.theme.space[3]}px;
  border-radius: 0;
`;
