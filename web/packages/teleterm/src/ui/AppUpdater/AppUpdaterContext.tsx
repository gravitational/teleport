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
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';

import { type AppUpdateEvent } from 'teleterm/services/appUpdater';
import { useAppContext } from 'teleterm/ui/appContextProvider';

interface AppUpdaterContext {
  updateEvent: AppUpdateEvent;
}

const AppUpdaterContext = createContext<AppUpdaterContext>(null);

export function AppUpdaterContextProvider(props: PropsWithChildren) {
  const appContext = useAppContext();
  const [updateEvent, setUpdateEvent] = useState<AppUpdateEvent>({
    kind: 'checking-for-update',
  });

  useEffect(() => {
    const { cleanup } =
      appContext.mainProcessClient.subscribeToAppUpdateEvents(setUpdateEvent);

    return cleanup;
  }, [appContext]);

  const value = useMemo(
    () => ({
      updateEvent,
    }),
    [updateEvent]
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
