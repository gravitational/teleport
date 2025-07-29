/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  createContext,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';

import { Platform } from 'teleterm/mainProcess/types';
import { type AppUpdateEvent } from 'teleterm/services/appUpdater';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { RootClusterUri } from 'teleterm/ui/uri';

interface AppUpdaterContext {
  updateEvent: AppUpdateEvent;
  platform: Platform;
  checkForUpdates(): Promise<void>;
  download(): void;
  cancelDownload(): void;
  quitAndInstall(): void;
  changeManagingCluster(clusterUri: RootClusterUri | undefined): Promise<void>;
}

const AppUpdaterContext = createContext<AppUpdaterContext>(null);

export function AppUpdaterContextProvider(props: PropsWithChildren) {
  const appContext = useAppContext();
  const [updateEvent, setUpdateEvent] = useState<AppUpdateEvent>({
    kind: 'checking-for-update',
  });

  const checkForUpdates = useCallback(async () => {
    try {
      await appContext.mainProcessClient.checkForAppUpdates();
    } catch {
      /* Empty - errors are read from updateEvent. */
    }
  }, [appContext]);

  const download = useCallback(async () => {
    try {
      await appContext.mainProcessClient.downloadAppUpdate();
    } catch {
      /* Empty - errors are read from updateEvent. */
    }
  }, [appContext]);

  const cancelDownload = useCallback(() => {
    appContext.mainProcessClient.cancelAppUpdateDownload();
  }, [appContext]);

  const quitAndInstall = useCallback(async () => {
    try {
      await appContext.mainProcessClient.quitAndInstallAppUpdate();
    } catch {
      /* Empty - errors are read from updateEvent. */
    }
  }, [appContext]);

  const changeManagingCluster = useCallback(
    async (clusterUri: RootClusterUri | undefined) => {
      try {
        await appContext.mainProcessClient.changeManagingCluster(clusterUri);
      } catch {
        /* Empty - errors are read from updateEvent. */
      }
    },
    [appContext]
  );

  useEffect(() => {
    const { cleanup } =
      appContext.mainProcessClient.subscribeToAppUpdateEvents(setUpdateEvent);

    return cleanup;
  }, [appContext]);

  const value = useMemo(
    () => ({
      updateEvent,
      platform: appContext.mainProcessClient.getRuntimeSettings().platform,
      checkForUpdates,
      download,
      cancelDownload,
      quitAndInstall,
      changeManagingCluster,
    }),
    [
      appContext.mainProcessClient,
      cancelDownload,
      changeManagingCluster,
      checkForUpdates,
      download,
      quitAndInstall,
      updateEvent,
    ]
  );

  return (
    <AppUpdaterContext.Provider value={value}>
      {props.children}
    </AppUpdaterContext.Provider>
  );
}

export const useAppUpdaterContext = () => {
  const context = useContext(AppUpdaterContext);

  if (!context) {
    throw new Error(
      'useAppUpdaterContext must be used within an AppUpdaterContext'
    );
  }

  return context;
};
