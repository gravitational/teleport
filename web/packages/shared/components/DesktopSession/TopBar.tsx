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

import { useTheme } from 'styled-components';

import { Flex, Text, TopNav } from 'design';
import { Clipboard } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import {
  SharedDirectoryList,
  DirectoryItem,
} from 'shared/components/DesktopSession/DirectoryList';
import { LatencyDiagnostic } from 'shared/components/LatencyDiagnostic';
import type { ToastNotificationItem } from 'shared/components/ToastNotification';

import ActionMenu from './ActionMenu';
import { AlertDropdown } from './AlertDropdown';

export default function TopBar(props: Props) {
  const {
    userHost,
    isSharingClipboard,
    clipboardSharingMessage,
    onDisconnect,
    canShareDirectory,
    onAddSharedDirectory,
    onCtrlAltDel,
    alerts,
    sharedDirectories,
    onRemoveAlert,
    onRemoveSharedDirectory,
    isConnected,
    latency,
    canRemoveSharedDirectory,
  } = props;
  const theme = useTheme();

  const primaryOnTrue = (b: boolean): any => {
    return {
      color: b ? theme.colors.text.main : theme.colors.text.disabled,
    };
  };

  return (
    <TopNav
      height="40px"
      bg="levels.deep"
      justifyContent="space-between"
      gap={3}
      px={3}
    >
      <Text style={{ color: theme.colors.text.slightlyMuted }}>{userHost}</Text>

      {isConnected && (
        <Flex gap={3} alignItems="center">
          {latency && <LatencyDiagnostic latency={latency} />}
          <HoverTooltip
            tipContent={directorySharingToolTip(
              canShareDirectory,
              sharedDirectories.length > 0 // isSharingDirectory
            )}
            placement="bottom"
          >
            <SharedDirectoryList
              sharedDirectories={sharedDirectories}
              onRemoveSharedDirectory={onRemoveSharedDirectory}
              onAddSharedDirectory={onAddSharedDirectory}
              canRemoveSharedDirectory={canRemoveSharedDirectory}
              canSharedDirectories={canShareDirectory}
            />
            {/*<FolderShared style={primaryOnTrue(isSharingDirectory)} />*/}
          </HoverTooltip>
          <HoverTooltip tipContent={clipboardSharingMessage} placement="bottom">
            <Clipboard style={primaryOnTrue(isSharingClipboard)} />
          </HoverTooltip>
          <AlertDropdown alerts={alerts} onRemoveAlert={onRemoveAlert} />
          <ActionMenu
            showShareDirectory={canShareDirectory}
            onShareDirectory={onAddSharedDirectory}
            onDisconnect={onDisconnect}
            onCtrlAltDel={onCtrlAltDel}
          />
        </Flex>
      )}
    </TopNav>
  );
}

function directorySharingToolTip(
  canShare: boolean,
  isSharing: boolean
): string {
  if (!canShare) {
    return 'Directory Sharing Disabled';
  }
  if (!isSharing) {
    return 'Add Shared Directory';
  }
  return 'Add or Remove Shared Directory';
}

type Props = {
  userHost: string;
  isSharingClipboard: boolean;
  clipboardSharingMessage: string;
  canShareDirectory: boolean;
  //isSharingDirectory: boolean;
  onDisconnect: VoidFunction;
  onAddSharedDirectory: VoidFunction;
  onCtrlAltDel: VoidFunction;
  alerts: ToastNotificationItem[];
  sharedDirectories: DirectoryItem[];
  isConnected: boolean;
  onRemoveAlert(id: string): void;
  onRemoveSharedDirectory(directoryId: number);
  latency: {
    client: number;
    server: number;
  };
  canRemoveSharedDirectory: boolean;
};
