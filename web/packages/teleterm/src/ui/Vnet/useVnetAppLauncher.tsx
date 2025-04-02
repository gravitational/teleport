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

import { useCallback, useMemo } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useConnectionsContext } from 'teleterm/ui/TopBar/Connections/connectionsContext';
import { ResourceUri, routing } from 'teleterm/ui/uri';

import { useVnetContext } from './vnetContext';

export type VnetAppLauncher = (args: {
  addrToCopy: string | undefined;
  /**
   * resourceUri lets the VNet launcher establish which workspace to open the info doc in if
   * there's a need to do it.
   */
  resourceUri: ResourceUri;
}) => Promise<void>;

export const useVnetAppLauncher = (): {
  /**
   * launchVnet is a function that manages VNet start when:
   *
   * - The user clicks "Connect" next to a TCP app or selects one of the ports from the menu.
   * - The user selects a TCP app through the search bar.
   *
   * If the user is yet to start VNet, it opens the info doc. If they already started it in the past,
   * it starts VNet and then copies the address of the app to the clipboard.
   */
  launchVnet: VnetAppLauncher;
  /**
   * launchVnetWithoutFirstTimeCheck never opens the info doc, it starts VNet and then copies the
   * address of the app to the clipboard.
   */
  launchVnetWithoutFirstTimeCheck: (
    addrToCopy: string | undefined
  ) => Promise<void>;
} => {
  const { notificationsService, workspacesService } = useAppContext();
  const { start, status, startAttempt, hasEverStarted } = useVnetContext();
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

  const openInfoDoc = useCallback(
    async ({
      addrToCopy,
      resourceUri,
    }: {
      addrToCopy: string | undefined;
      resourceUri: ResourceUri;
    }) => {
      const rootClusterUri = routing.ensureRootClusterUri(resourceUri);
      // Since VNet app launcher might be called from the search bar, we have to account for the
      // user being in a different workspace than the selected app.
      const { isAtDesiredWorkspace } =
        await workspacesService.setActiveWorkspace(rootClusterUri);

      if (!isAtDesiredWorkspace) {
        return;
      }

      const docsService =
        workspacesService.getWorkspaceDocumentService(rootClusterUri);
      const docUri = docsService.openExistingOrAddNew(
        d => d.kind === 'doc.vnet_info',
        () =>
          docsService.createVnetInfoDocument({
            rootClusterUri,
          })
      );
      // Update targetAddress so that clicking "Start VNet" from the info doc is going to copy that
      // address to clipboard.
      docsService.update(docUri, { app: { targetAddress: addrToCopy } });
    },
    [workspacesService]
  );

  const launchVnetAndCopyAddr = useCallback(
    async (addrToCopy: string | undefined) => {
      if (!(await launchVnet())) {
        return;
      }

      if (!addrToCopy) {
        return;
      }

      const ok = copyAddrToClipboard(addrToCopy);
      notificationsService.notifyInfo(
        ok
          ? `Connect via VNet by using ${addrToCopy} (copied to clipboard).`
          : `Connect via VNet by using ${addrToCopy}.`
      );
    },
    [launchVnet, notificationsService]
  );

  return useMemo(
    () => ({
      launchVnet: async args => {
        if (!hasEverStarted) {
          openInfoDoc(args);
          return;
        }

        await launchVnetAndCopyAddr(args.addrToCopy);
      },
      launchVnetWithoutFirstTimeCheck: launchVnetAndCopyAddr,
    }),
    [hasEverStarted, openInfoDoc, launchVnetAndCopyAddr]
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
