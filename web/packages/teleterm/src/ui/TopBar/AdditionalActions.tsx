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

import { useRef, useState } from 'react';

import { Popover } from 'design';
import * as icons from 'design/Icon';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceServiceState } from 'teleterm/ui/services/workspacesService';
import { useNewTabOpener } from 'teleterm/ui/TabHost';
import { TopBarButton } from 'teleterm/ui/TopBar/TopBarButton';

import { Menu, MenuItem, MenuListItem } from '../components/Menu';

function useMenuItems(): MenuItem[] {
  const ctx = useAppContext();
  const { workspacesService, mainProcessClient, notificationsService } = ctx;
  useWorkspaceServiceState();
  const documentsService =
    workspacesService.getActiveWorkspaceDocumentService();
  const { openTerminalTab } = useNewTabOpener({
    documentsService,
    localClusterUri: workspacesService.getActiveWorkspace()?.localClusterUri,
  });

  const hasNoActiveWorkspace = !documentsService;

  const { platform } = mainProcessClient.getRuntimeSettings();
  const isDarwin = platform === 'darwin';

  const menuItems: (MenuItem & { isVisible: boolean })[] = [
    {
      title: 'Open new terminal',
      isVisible: true,
      isDisabled: hasNoActiveWorkspace,
      disabledText:
        'You need to be logged in to a cluster to open new terminals.',
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
  ];

  return menuItems.filter(i => i.isVisible);
}

export function AdditionalActions() {
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const selectorRef = useRef<HTMLButtonElement>(null);

  const items = useMenuItems().map(item => {
    return (
      <MenuListItem
        key={item.title}
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
        title="Additional Actions"
        onClick={() => setIsPopoverOpened(true)}
      >
        <icons.MoreVert size="medium" />
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
