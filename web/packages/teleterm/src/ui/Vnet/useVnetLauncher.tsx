/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useCallback } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useConnectionsContext } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { useVnetContext } from './vnetContext';

/**
 * VnetLauncher is a function that manages VNet start when:
 *
 * - The user clicks "Connect" next to a TCP app or selects one of the ports from the menu.
 * - The user selects a TCP app through the search bar.
 */
export type VnetLauncher = (addrToCopy: string) => Promise<void>;

export const useVnetLauncher = (): VnetLauncher => {
  const { notificationsService } = useAppContext();
  const { start, status, startAttempt } = useVnetContext();
  const { open } = useConnectionsContext();

  const launchVnet: () => Promise<boolean> = useCallback(async () => {
    if (status.value === 'running' || startAttempt.status === 'processing') {
      return true;
    }

    open('vnet');

    const [, error] = await start();
    if (error) {
      // The error is going to be shown in the VNet panel that was just opened.
      return false;
    }
    return true;
  }, [status.value, startAttempt.status, open, start]);

  return useCallback(
    async (addrToCopy: string) => {
      if (!(await launchVnet())) {
        return;
      }

      const ok = copyAddrToClipboard(addrToCopy);
      notificationsService.notifyInfo(
        ok
          ? `Connect via VNet by using ${addrToCopy} (copied to clipboard).`
          : `Connect via VNet by using ${addrToCopy}.`
      );
    },
    [notificationsService, launchVnet]
  );
};

/**
 * Returns true if address was copied to clipboard.
 */
const copyAddrToClipboard = async (addrToCopy: string): Promise<boolean> => {
  try {
    await navigator.clipboard.writeText(addrToCopy);
  } catch (error) {
    // On macOS, if the user starts VNet for the first time, the app does not have focus after
    // approving the background item. Chromium has strict rules regarding when JS can access the
    // clipboard, so it'll throw an error. https://github.com/gravitational/teleport/issues/53290
    //
    // Similarly, in dev mode if the user uses the mouse rather than the keyboard to proceed with
    // the osascript prompt, there will also be a problem with focus.
    if (error['name'] === 'NotAllowedError') {
      console.error(error);
      return false;
    }
    throw error;
  }
  return true;
};
