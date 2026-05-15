/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import styled, { useTheme } from 'styled-components';

import { Box, Flex, ResourceIcon } from 'design';
import { Clipboard, FolderShared } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import ActionMenu from 'shared/components/DesktopSession/ActionMenu';
import { AlertDropdown } from 'shared/components/DesktopSession/AlertDropdown';
import type { DesktopSessionControlsRenderProps } from 'shared/components/DesktopSession/DesktopSession';
import { LatencyDiagnostic } from 'shared/components/LatencyDiagnostic';

import { statusBarHeight } from './constants';

export function DesktopSessionControls({
  controls,
}: {
  controls: DesktopSessionControlsRenderProps;
}) {
  const theme = useTheme();

  const iconColor = (active: boolean) =>
    active ? theme.colors.text.main : theme.colors.text.muted;

  return (
    <Inset>
      <Box mx={2}>
        <ResourceIcon name="windows" size="large" />
      </Box>
      {controls.latencyStats && (
        <LatencyDiagnostic latency={controls.latencyStats} />
      )}
      <HoverTooltip
        tipContent={directorySharingTooltip(
          controls.canShareDirectory,
          controls.isSharingDirectory
        )}
        placement="top"
      >
        <FolderShared
          size="small"
          padding="8px"
          color={iconColor(controls.isSharingDirectory)}
        />
      </HoverTooltip>
      <HoverTooltip
        tipContent={controls.clipboardSharingMessage}
        placement="top"
      >
        <Clipboard
          size="small"
          padding="8px"
          color={iconColor(controls.isSharingClipboard)}
        />
      </HoverTooltip>
      {!!controls?.alerts?.length && (
        <AlertDropdown
          alerts={controls.alerts}
          onRemoveAlert={controls.onRemoveAlert}
          top="unset"
          bottom={statusBarHeight + theme.space[2]}
          right={2}
        />
      )}
      <Divider />
      <ActionMenu
        showShareDirectory={
          controls.canShareDirectory && !controls.isSharingDirectory
        }
        onShareDirectory={controls.onShareDirectory}
        onCtrlAltDel={controls.onCtrlAltDel}
        onDisconnect={controls.onDisconnect}
        openUpward
        buttonIconColor="text.slightlyMuted"
      />
    </Inset>
  );
}

const Inset = styled(Flex).attrs({ alignSelf: 'center', alignItems: 'center' })`
  background: ${({ theme }) => theme.colors.levels.sunken};
  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2) inset,
    0 1px 1px 0 rgba(0, 0, 0, 0.14) inset,
    0 1px 3px 0 rgba(0, 0, 0, 0.12) inset;
  border-radius: ${({ theme }) => theme.radii[3]}px;
  height: 32px;
  margin: 4px auto;
  gap: 2px;
`;

const Divider = styled.div`
  width: 1px;
  height: 20px;
  background: ${({ theme }) => theme.colors.interactive.tonal.neutral[1]};
`;

function directorySharingTooltip(
  canShare: boolean,
  isSharing: boolean
): string {
  if (!canShare) {
    return 'Directory Sharing Disabled';
  }
  if (!isSharing) {
    return 'Directory Sharing Inactive';
  }
  return 'Directory Sharing Enabled';
}
