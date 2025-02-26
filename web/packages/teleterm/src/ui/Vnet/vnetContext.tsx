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
  createContext,
  FC,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';

import { BackgroundItemStatus } from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';
import { Report } from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import { Attempt, makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import { isTshdRpcError } from 'teleterm/services/tshd';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { usePersistedState } from 'teleterm/ui/hooks/usePersistedState';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { IAppContext } from 'teleterm/ui/types';

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
  listDNSZones: () => Promise<[string[], Error]>;
  listDNSZonesAttempt: Attempt<string[]>;
  runDiagnostics: () => Promise<[Report, Error]>;
  diagnosticsAttempt: Attempt<Report>;
  resetDiagnosticsAttempt: () => void;
  /**
   * Calculates whether the button for running diagnostics should be disabled. If it should be
   * disabled, it returns a reason for this, otherwise it returns a falsy value.
   *
   * Accepts an attempt as an arg to accommodate for places that run diagnostics periodically
   * vs manually.
   */
  getDisabledDiagnosticsReason: (
    runDiagnosticsAttempt: Attempt<Report>
  ) => string;
};

export type VnetStatus =
  | { value: 'running' }
  | { value: 'stopped'; reason: VnetStoppedReason };

export type VnetStoppedReason =
  | { value: 'regular-shutdown-or-not-started' }
  | { value: 'unexpected-shutdown'; errorMessage: string };

export const VnetContext = createContext<VnetContext>(null);

export const VnetContextProvider: FC<PropsWithChildren> = props => {
  const [status, setStatus] = useState<VnetStatus>({
    value: 'stopped',
    reason: { value: 'regular-shutdown-or-not-started' },
  });
  const appCtx = useAppContext();
  const { vnet, mainProcessClient, notificationsService } = appCtx;
  const isWorkspaceStateInitialized = useStoreSelector(
    'workspacesService',
    useCallback(state => state.isInitialized, [])
  );
  const [{ autoStart }, setAppState] = usePersistedState('vnet', {
    autoStart: false,
  });

  const isSupported = useMemo(() => {
    const { platform } = mainProcessClient.getRuntimeSettings();
    return platform === 'darwin' || platform === 'win32';
  }, [mainProcessClient]);

  const [startAttempt, start] = useAsync(
    useCallback(async () => {
      await notifyAboutDaemonBackgroundItem(appCtx);

      try {
        await vnet.start({});
      } catch (error) {
        if (!isTshdRpcError(error, 'ALREADY_EXISTS')) {
          throw error;
        }
      }
      setStatus({ value: 'running' });
      setAppState({ autoStart: true });
    }, [vnet, setAppState, appCtx])
  );

  const [diagnosticsAttempt, runDiagnostics, setDiagnosticsAttempt] = useAsync(
    useCallback(
      () => vnet.runDiagnostics({}).then(({ response }) => response.report),
      [vnet]
    )
  );
  const resetDiagnosticsAttempt = useCallback(
    () => setDiagnosticsAttempt(makeEmptyAttempt()),
    [setDiagnosticsAttempt]
  );

  const [stopAttempt, stop] = useAsync(
    useCallback(async () => {
      await vnet.stop({});
      setStatus({
        value: 'stopped',
        reason: { value: 'regular-shutdown-or-not-started' },
      });
      setAppState({ autoStart: false });
    }, [vnet, setAppState])
  );

  const [listDNSZonesAttempt, listDNSZones] = useAsync(
    useCallback(
      () => vnet.listDNSZones({}).then(({ response }) => response.dnsZones),
      [vnet]
    )
  );

  /**
   * Calculates whether the button for running diagnostics should be disabled. If it should be
   * disabled, it returns a reason for this, otherwise it returns a falsy value.
   *
   * Accepts an attempt as an arg to accommodate for places that run diagnostics periodically
   * vs manually.
   */
  const getDisabledDiagnosticsReason = useCallback(
    (runDiagnosticsAttempt: Attempt<Report>) =>
      status.value !== 'running'
        ? 'VNet must be running to run diagnostics'
        : runDiagnosticsAttempt.status === 'processing'
          ? 'Generating diagnostic report…'
          : '',
    [status.value]
  );
  useEffect(() => {
    const handleAutoStart = async () => {
      if (
        isSupported &&
        autoStart &&
        // Accessing resources through VNet might trigger the MFA modal,
        // so we have to wait for the tshd events service to be initialized.
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

  useEffect(
    function handleUnexpectedShutdown() {
      const removeListener = appCtx.addUnexpectedVnetShutdownListener(
        ({ error }) => {
          setStatus({
            value: 'stopped',
            reason: { value: 'unexpected-shutdown', errorMessage: error },
          });

          notificationsService.notifyError({
            title: 'VNet has unexpectedly shut down',
            description: error
              ? `Reason: ${error}`
              : 'No reason was given, check the logs for more details.',
          });
        }
      );

      return removeListener;
    },
    [appCtx, notificationsService]
  );

  return (
    <VnetContext.Provider
      value={{
        isSupported,
        status,
        start,
        startAttempt,
        stop,
        stopAttempt,
        listDNSZones,
        listDNSZonesAttempt,
        runDiagnostics,
        diagnosticsAttempt,
        resetDiagnosticsAttempt,
        getDisabledDiagnosticsReason,
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

const notifyAboutDaemonBackgroundItem = async (ctx: IAppContext) => {
  const { vnet, notificationsService } = ctx;

  let backgroundItemStatus: BackgroundItemStatus;
  try {
    const { response } = await vnet.getBackgroundItemStatus({});
    backgroundItemStatus = response.status;
  } catch (error) {
    // vnet.getBackgroundItemStatus returns UNIMPLEMENTED if tsh was compiled without the
    // vnetdaemon build tag.
    if (isTshdRpcError(error, 'UNIMPLEMENTED')) {
      return;
    }

    throw error;
  }

  if (
    backgroundItemStatus === BackgroundItemStatus.ENABLED ||
    backgroundItemStatus === BackgroundItemStatus.NOT_SUPPORTED ||
    backgroundItemStatus === BackgroundItemStatus.UNSPECIFIED
  ) {
    return;
  }

  notificationsService.notifyInfo(
    'Please enable the background item for tsh.app in System Settings > General > Login Items to start VNet.'
  );
};
