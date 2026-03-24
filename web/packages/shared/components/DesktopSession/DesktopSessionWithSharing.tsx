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

import React from 'react';

import { Attempt } from 'shared/hooks/useAsync';
import { TdpClient } from 'shared/libs/tdp';

import { DesktopSession } from './DesktopSession';
import { DesktopSessionControlsRenderProps } from './DesktopSession';
import TopBar from './TopBar';

export type DesktopSessionWithSharingProps = {
  client: TdpClient;
  username: string;
  desktop: string;
  aclAttempt: Attempt<{
    clipboardSharingEnabled: boolean;
    directorySharingEnabled: boolean;
  }>;
  /** Determines if the browser client support directory and clipboard sharing. */
  browserSupportsSharing: boolean;
  customConnectionState?(args: { retry(): void }): React.ReactElement;
  hasAnotherSession(): Promise<boolean>;
  keyboardLayout?: number;
};

/**
 * Composes DesktopSession with useDesktopSession for use in the web UI.
 * Teleport Connect calls useDesktopSession directly in DocumentDesktopSession
 * so it can publish session state to the status bar.
 */
export function DesktopSessionWithSharing({
  ...props
}: DesktopSessionWithSharingProps) {
  return (
    <DesktopSession
      {...props}
      renderControls={(controls: DesktopSessionControlsRenderProps) => (
        <TopBar
          isConnected={controls.isConnected}
          onDisconnect={controls.onDisconnect}
          userHost={`${props.username} on ${props.desktop}`}
          canShareDirectory={controls.canShareDirectory}
          isSharingDirectory={controls.isSharingDirectory}
          isSharingClipboard={controls.isSharingClipboard}
          clipboardSharingMessage={controls.clipboardSharingMessage}
          onShareDirectory={controls.onShareDirectory}
          onCtrlAltDel={controls.onCtrlAltDel}
          alerts={controls.alerts}
          onRemoveAlert={controls.onRemoveAlert}
          latency={controls.latencyStats}
        />
      )}
    />
  );
}
