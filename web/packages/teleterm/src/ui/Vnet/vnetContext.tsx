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

import {
  FC,
  PropsWithChildren,
  createContext,
  useContext,
  useState,
  useCallback,
  useMemo,
  useEffect,
} from 'react';
import { useAsync, Attempt } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { usePersistedState } from 'teleterm/ui/hooks/usePersistedState';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';

/**
 * VnetContext manages the VNet instance.
 *
 * There is a single VNet instance running for all workspaces.
 */
export type VnetContext = {
  /**
   * Describes whether the given OS can run VNet.
   */
  isSupported: boolean;
  status: VnetStatus;
  start: () => Promise<[void, Error]>;
  startAttempt: Attempt<void>;
  stop: () => Promise<[void, Error]>;
  stopAttempt: Attempt<void>;
};

export type VnetStatus = 'running' | 'stopped';

export const VnetContext = createContext<VnetContext>(null);

export const VnetContextProvider: FC<PropsWithChildren> = props => {
  const [status, setStatus] = useState<VnetStatus>('stopped');
  const { vnet, mainProcessClient, configService } = useAppContext();
  const isWorkspaceStateInitialized = useStoreSelector(
    'workspacesService',
    useCallback(state => state.isInitialized, [])
  );
  const [{ autoStart }, setAppState] = usePersistedState('vnet', {
    autoStart: false,
  });

  const isSupported = useMemo(
    () =>
      mainProcessClient.getRuntimeSettings().platform === 'darwin' &&
      configService.get('feature.vnet').value,
    [mainProcessClient, configService]
  );

  const [startAttempt, start] = useAsync(
    useCallback(async () => {
      // TODO(ravicious): If the osascript dialog was canceled, do not throw an error and instead
      // just don't update status. Perhaps even revert back attempt status if possible.
      //
      // Reconsider this only once the VNet daemon gets added.
      await vnet.start({});
      setStatus('running');
      setAppState({ autoStart: true });
    }, [vnet, setAppState])
  );

  const [stopAttempt, stop] = useAsync(
    useCallback(async () => {
      await vnet.stop({});
      setStatus('stopped');
      setAppState({ autoStart: false });
    }, [vnet, setAppState])
  );

  useEffect(() => {
    const handleAutoStart = async () => {
      if (
        isSupported &&
        autoStart &&
        isWorkspaceStateInitialized &&
        startAttempt.status === ''
      ) {
        const [, error] = await start();

        // Turn off autostart if starting fails. Otherwise the user wouldn't be able to turn off
        // autostart by themselves.
        if (error) {
          setAppState({ autoStart: false });
        }
      }
    };

    handleAutoStart();
  }, [isWorkspaceStateInitialized]);

  return (
    <VnetContext.Provider
      value={{
        isSupported,
        status,
        start,
        startAttempt,
        stop,
        stopAttempt,
      }}
    >
      {props.children}
    </VnetContext.Provider>
  );
};

export const useVnetContext = () => {
  const context = useContext(VnetContext);

  if (!context) {
    throw new Error('useVnetContext must be used within a VnetContextProvider');
  }

  return context;
};
