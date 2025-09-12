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

import { deserializeError } from 'teleterm/mainProcess/ipcSerializer';
import { type AppUpdateEvent } from 'teleterm/services/appUpdater';
import { useAppContext } from 'teleterm/ui/appContextProvider';

interface AppUpdaterContext {
  updateEvent: AppUpdateEvent;
  openDialog(): void;
}

const AppUpdaterContext = createContext<AppUpdaterContext>(null);

export function AppUpdaterContextProvider(props: PropsWithChildren) {
  const appContext = useAppContext();
  const [updateEvent, setUpdateEvent] = useState<AppUpdateEvent>({
    kind: 'checking-for-update',
  });

  const openDialog = useCallback(() => {
    appContext.modalsService.openRegularDialog({ kind: 'app-updates' });
  }, [appContext]);

  useEffect(() => {
    const { cleanup: cleanUpEvents } =
      appContext.mainProcessClient.subscribeToAppUpdateEvents(event => {
        if (event.kind === 'error') {
          event.error = deserializeError(event.error);
        }
        setUpdateEvent(event);
      });
    const { cleanup: cleanUpDialog } =
      appContext.mainProcessClient.subscribeToOpenAppUpdateDialog(openDialog);

    return () => {
      cleanUpEvents();
      cleanUpDialog();
    };
  }, [appContext, openDialog]);

  const value = useMemo(
    () => ({
      updateEvent,
      openDialog,
    }),
    [updateEvent, openDialog]
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
