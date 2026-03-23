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

import type { ToastNotificationItem } from 'shared/components/ToastNotification';

/** Session state published by DocumentDesktopSession and consumed by the status bar. */
export type DesktopSessionStatusBarState = {
  isSharingClipboard: boolean;
  clipboardSharingMessage: string;
  canShareDirectory: boolean;
  isSharingDirectory: boolean;
  onShareDirectory: VoidFunction;
  onCtrlAltDel: VoidFunction;
  onDisconnect: VoidFunction;
  alerts: ToastNotificationItem[];
  onRemoveAlert(id: string): void;
  isConnected: boolean;
};

type Listener = () => void;

/**
 * Holds the state of the currently visible desktop session so the
 * status bar can display session controls.
 */
export class ActiveDesktopSessionService {
  private _state: DesktopSessionStatusBarState | null = null;
  private _subscribers = new Set<Listener>();

  setState(state: DesktopSessionStatusBarState | null): void {
    this._state = state;
    this._subscribers.forEach(s => s());
  }

  getState(): DesktopSessionStatusBarState | null {
    return this._state;
  }

  subscribe(listener: Listener): () => void {
    this._subscribers.add(listener);
    return () => this._subscribers.delete(listener);
  }
}
