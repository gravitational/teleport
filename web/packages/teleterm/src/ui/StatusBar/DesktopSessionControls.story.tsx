/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { DesktopSessionControlsRenderProps } from 'shared/components/DesktopSession/DesktopSession';
import { ToastNotificationItem } from 'shared/components/ToastNotification/types';

import { DesktopSessionControls } from './DesktopSessionControls';

export default {
  title: 'Teleterm/StatusBar/DesktopSessionControls',
  parameters: {
    layout: 'centered',
  },
};

const controls: DesktopSessionControlsRenderProps = {
  canShareDirectory: true,
  isSharingDirectory: false,
  isSharingClipboard: false,
  clipboardSharingMessage: 'Clipboard sharing inactive.',
  onShareDirectory: () => {},
  onCtrlAltDel: () => {},
  onDisconnect: () => {},
  onRemoveAlert: () => {},
  alerts: [],
  isConnected: false,
  latencyStats: undefined,
};

export function NoAlerts() {
  return <DesktopSessionControls controls={controls} />;
}

export function WithAlert() {
  const alerts = [
    {
      severity: 'warn',
      content: 'This is a warning message.',
      id: 'warning-1',
    },
  ] as ToastNotificationItem[];
  const alertControls = { ...controls, alerts };
  return <DesktopSessionControls controls={alertControls} />;
}
