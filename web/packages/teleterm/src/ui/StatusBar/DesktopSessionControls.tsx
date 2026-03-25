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

import { Flex } from 'design';
import { Clipboard, FolderShared } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import ActionMenu from 'shared/components/DesktopSession/ActionMenu';
import { AlertDropdown } from 'shared/components/DesktopSession/AlertDropdown';
import type { DesktopSessionControlsRenderProps } from 'shared/components/DesktopSession/DesktopSession';
import { LatencyDiagnostic } from 'shared/components/LatencyDiagnostic';

export function DesktopSessionControls({
  status,
}: {
  status: DesktopSessionControlsRenderProps;
}) {
  const theme = useTheme();

  const primaryOnTrue = (active: boolean) => ({
    color: active
      ? theme.colors.text.primaryInverse
      : `${theme.colors.text.primaryInverse}80`,
  });

  return (
    <Pill alignItems="center" gap={2}>
      {status.latencyStats && <LatencyDiagnostic latency={status.latencyStats} />}
      <HoverTooltip
        tipContent={directorySharingTooltip(
          status.canShareDirectory,
          status.isSharingDirectory
        )}
        placement="top"
      >
        <FolderShared
          size="small"
          style={primaryOnTrue(status.isSharingDirectory)}
        />
      </HoverTooltip>
      <HoverTooltip tipContent={status.clipboardSharingMessage} placement="top">
        <Clipboard
          size="small"
          style={primaryOnTrue(status.isSharingClipboard)}
        />
      </HoverTooltip>
      <AlertDropdown
        alerts={status.alerts}
        onRemoveAlert={status.onRemoveAlert}
        openUpward
        iconColor={theme.colors.text.primaryInverse}
        noAlertsBackground="transparent"
      />
      <ActionMenu
        showShareDirectory={
          status.canShareDirectory && !status.isSharingDirectory
        }
        onShareDirectory={status.onShareDirectory}
        onCtrlAltDel={status.onCtrlAltDel}
        onDisconnect={status.onDisconnect}
        openUpward
        buttonIconColor="text.primaryInverse"
      />
    </Pill>
  );
}

const Pill = styled(Flex)`
  background: ${({ theme }) => theme.colors.interactive.solid.primary.default};
  border-radius: ${({ theme }) => theme.radii[2]}px;
  padding: 0 ${({ theme }) => theme.space[2]}px;
  height: 100%;
  position: relative;
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
