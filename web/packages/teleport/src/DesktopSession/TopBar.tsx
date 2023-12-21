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

import React from 'react';
import { useTheme } from 'styled-components';
import { Text, TopNav, Flex } from 'design';
import { Clipboard, FolderShared } from 'design/Icon';

import ActionMenu from './ActionMenu';
import { WarningDropdown } from './WarningDropdown';

import type { NotificationItem } from 'shared/components/Notification';

export default function TopBar(props: Props) {
  const {
    userHost,
    clipboardSharingEnabled,
    onDisconnect,
    canShareDirectory,
    isSharingDirectory,
    onShareDirectory,
    warnings,
    onRemoveWarning,
  } = props;
  const theme = useTheme();

  const primaryOnTrue = (b: boolean): any => {
    return {
      color: b ? theme.colors.text.main : theme.colors.text.slightlyMuted,
    };
  };

  return (
    <TopNav
      height={`${TopBarHeight}px`}
      bg="levels.deep"
      style={{
        justifyContent: 'space-between',
      }}
    >
      <Text px={3} style={{ color: theme.colors.text.slightlyMuted }}>
        {userHost}
      </Text>

      <Flex px={3}>
        <Flex alignItems="center">
          <FolderShared
            style={primaryOnTrue(isSharingDirectory)}
            pr={3}
            title={directorySharingTitle(canShareDirectory, isSharingDirectory)}
          />
          <Clipboard
            style={primaryOnTrue(clipboardSharingEnabled)}
            pr={3}
            title={
              clipboardSharingEnabled
                ? 'Clipboard Sharing Enabled'
                : 'Clipboard Sharing Disabled'
            }
          />
          <WarningDropdown
            warnings={warnings}
            onRemoveWarning={onRemoveWarning}
          />
        </Flex>
        <ActionMenu
          onDisconnect={onDisconnect}
          showShareDirectory={canShareDirectory && !isSharingDirectory}
          onShareDirectory={onShareDirectory}
        />
      </Flex>
    </TopNav>
  );
}

function directorySharingTitle(canShare: boolean, isSharing: boolean): string {
  if (!canShare) {
    return 'Directory Sharing Disabled';
  }
  if (!isSharing) {
    return 'Directory Sharing Inactive';
  }
  return 'Directory Sharing Enabled';
}

export const TopBarHeight = 40;

type Props = {
  userHost: string;
  clipboardSharingEnabled: boolean;
  canShareDirectory: boolean;
  isSharingDirectory: boolean;
  onDisconnect: VoidFunction;
  onShareDirectory: VoidFunction;
  warnings: NotificationItem[];
  onRemoveWarning(id: string): void;
};
