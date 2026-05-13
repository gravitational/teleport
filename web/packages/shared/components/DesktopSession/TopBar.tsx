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
import styled from 'styled-components';

import { Flex, Text, TopNav } from 'design';
import { Clipboard } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { SessionSettings } from 'shared/components/DesktopSession/SessionSettings';
import {
  DirectoryItem,
  SharedDirectoryList,
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
    hiDpiEnabled,
    onToggleHiDpi,
    screenIsHiDpi,
    hiDpiSupported,
    canRemoveSharedDirectory,
    maxSharedDirectories,
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
          <SharedDirectoryList
            sharedDirectories={sharedDirectories}
            onRemoveSharedDirectory={onRemoveSharedDirectory}
            onAddSharedDirectory={onAddSharedDirectory}
            canRemoveSharedDirectory={canRemoveSharedDirectory}
            canSharedDirectories={canShareDirectory}
            maxSharedDirectories={maxSharedDirectories}
          />
          <HoverTooltip tipContent={clipboardSharingMessage} placement="bottom">
            <Clipboard style={primaryOnTrue(isSharingClipboard)} />
          </HoverTooltip>
          {!!alerts?.length && (
            <AlertDropdown alerts={alerts} onRemoveAlert={onRemoveAlert} />
          )}
          <Divider />
          <SessionSettings
            hiDpiEnabled={hiDpiEnabled}
            onToggleHiDpi={onToggleHiDpi}
            screenIsHiDpi={screenIsHiDpi}
            hiDpiSupported={hiDpiSupported}
          />
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

type Props = {
  userHost: string;
  isSharingClipboard: boolean;
  clipboardSharingMessage: string;
  canShareDirectory: boolean;
  onDisconnect: VoidFunction;
  onAddSharedDirectory: VoidFunction;
  onCtrlAltDel: VoidFunction;
  alerts: ToastNotificationItem[];
  sharedDirectories: DirectoryItem[];
  isConnected: boolean;
  hiDpiEnabled: boolean;
  onToggleHiDpi: VoidFunction;
  screenIsHiDpi: boolean;
  hiDpiSupported: boolean;
  onRemoveAlert(id: string): void;
  onRemoveSharedDirectory(directoryId: number);
  latency: {
    client: number;
    server: number;
  };
  canRemoveSharedDirectory: boolean;
  maxSharedDirectories: number;
};

const Divider = styled.div`
  width: 1px;
  height: 24px;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[2]};
  margin-right: -${p => p.theme.space[1]}px; // avoid large visual gap between divider and next icon
`;
