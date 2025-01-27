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
  useRef,
  useState,
} from 'react';

import { BackgroundItemStatus } from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';
import { Report } from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import { useStateRef } from 'shared/hooks';
import { Attempt, makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import { cloneAbortSignal, isTshdRpcError } from 'teleterm/services/tshd';
import { hasReportFoundIssues } from 'teleterm/services/vnet/diag';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { usePersistedState } from 'teleterm/ui/hooks/usePersistedState';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { useConnectionsContext } from 'teleterm/ui/TopBar/Connections/connectionsContext';
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
  /**
   * Dismisses the diagnostics alert shown in the VNet panel. It won't be shown again until the user
   * reinstates the alert by manually requesting diagnostics to be run from the VNet panel.
   *
   * The user can dismissed an alert only after a diagnostics run was successful and either found
   * some issues or some checks have failed to complete.
   */
  dismissDiagnosticsAlert: () => void;
  /**
   * Whether the user dismissed the diagnostics alert in the VNet panel.
   */
  hasDismissedDiagnosticsAlert: boolean;
  /**
   * Shows the diagnostics alert in the VNet panel again.
   */
  reinstateDiagnosticsAlert: () => void;
  /**
   * openReport opens the report in a new document. If there's already a document with the same
   * report, it opens the existing document instead.
   *
   * openReport is undefined if the user is not within any workspace.
   */
  openReport: ((report: Report) => void) | undefined;
};

export type VnetStatus =
  | { value: 'running' }
  | { value: 'stopped'; reason: VnetStoppedReason };

export type VnetStoppedReason =
  | { value: 'regular-shutdown-or-not-started' }
  | { value: 'unexpected-shutdown'; errorMessage: string };

export const VnetContext = createContext<VnetContext>(null);

export const VnetContextProvider: FC<
  PropsWithChildren<{ diagnosticsIntervalMs?: number }>
> = ({ diagnosticsIntervalMs = defaultDiagnosticsIntervalMs, children }) => {
  const [status, setStatus] = useState<VnetStatus>({
    value: 'stopped',
    reason: { value: 'regular-shutdown-or-not-started' },
  });
  const appCtx = useAppContext();
  const {
    vnet,
    mainProcessClient,
    notificationsService,
    workspacesService,
    configService,
  } = appCtx;
  const isWorkspaceStateInitialized = useStoreSelector(
    'workspacesService',
    useCallback(state => state.isInitialized, [])
  );
  const [{ autoStart }, setAppState] = usePersistedState('vnet', {
    autoStart: false,
  });
  const { isOpenRef: isConnectionsPanelOpenRef } = useConnectionsContext();

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
      (signal?: AbortSignal) =>
        vnet
          .runDiagnostics({}, { abort: signal && cloneAbortSignal(signal) })
          .then(({ response }) => response.report),
      [vnet]
    )
  );

  /** Holds the ID of the currently displayed warning notification about diagnostics. */
  const diagNotificationIdRef = useRef('');
  /**
   * Removes any currently shown diag notification _and_ makes it so that the next periodic call to
   * runDiagnosticsAndShowNotification is not going to skip the notification due to the user
   * interacting with the notification.
   */
  const resetHasActedOnPreviousNotification = useCallback(() => {
    if (!diagNotificationIdRef.current) {
      return;
    }
    notificationsService.removeNotification(diagNotificationIdRef.current);
    diagNotificationIdRef.current = '';
  }, [notificationsService]);

  const [
    /** Whether user has dismissed the diagnostic alert shown in the VNet panel. */
    hasDismissedDiagnosticsAlert,
    hasDismissedDiagnosticsAlertRef,
    setHasDismissedDiagnosticsAlert,
  ] = useStateRef(false);
  const reinstateDiagnosticsAlert = useCallback(() => {
    setHasDismissedDiagnosticsAlert(false);
  }, [setHasDismissedDiagnosticsAlert]);
  const dismissDiagnosticsAlert = useCallback(() => {
    setHasDismissedDiagnosticsAlert(true);
    resetHasActedOnPreviousNotification();
  }, [setHasDismissedDiagnosticsAlert, resetHasActedOnPreviousNotification]);

  const [stopAttempt, stop] = useAsync(
    useCallback(async () => {
      await vnet.stop({});
      setStatus({
        value: 'stopped',
        reason: { value: 'regular-shutdown-or-not-started' },
      });
      setAppState({ autoStart: false });
      setDiagnosticsAttempt(makeEmptyAttempt());
      setHasDismissedDiagnosticsAlert(false);
    }, [
      vnet,
      setAppState,
      setDiagnosticsAttempt,
      setHasDismissedDiagnosticsAlert,
    ])
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

  const rootClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(state => state.rootClusterUri, [])
  );

  const openReport = useCallback(
    (report: Report) => {
      if (!rootClusterUri) {
        return;
      }

      const docsService =
        workspacesService.getWorkspaceDocumentService(rootClusterUri);

      // Check for an existing doc first. It may be present if someone re-runs diagnostics from within
      // a doc, then opens the VNet panel and clicks "Open Diag Report". The report in the panel and
      // the report in the doc are equal in that case, as they both come from diagnosticsAttempt.data.
      const existingDoc = docsService.getDocuments().find(
        d =>
          d.kind === 'doc.vnet_diag_report' &&
          // Reports don't have IDs, so createdAt is used as a good-enough approximation of an ID.
          d.report?.createdAt === report.createdAt
      );
      if (existingDoc) {
        docsService.open(existingDoc.uri);
      } else {
        const doc = docsService.createVnetDiagReportDocument({
          rootClusterUri,
          report,
        });
        docsService.add(doc);
        docsService.open(doc.uri);
      }

      // NOTE: Do not reset diagNotificationIdRef here for the notification to be considered acted
      // upon on the next run of runDiagnosticsAndShowNotification.
      notificationsService.removeNotification(diagNotificationIdRef.current);
    },
    [rootClusterUri, workspacesService, notificationsService]
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

  const runDiagnosticsAndShowNotification = useCallback(
    async (signal: AbortSignal) => {
      const [report, error] = await runDiagnostics(signal);
      if (error) {
        return;
      }

      const previousNotificationId = diagNotificationIdRef.current;
      /**
       * Whether the user dismissed the previous notification or opened the report from the previous
       * notification.
       */
      const hasActedOnPreviousNotification =
        previousNotificationId &&
        !notificationsService.hasNotification(previousNotificationId);
      notificationsService.removeNotification(previousNotificationId);

      if (
        hasActedOnPreviousNotification ||
        hasDismissedDiagnosticsAlertRef.current
      ) {
        return;
      }

      if (!hasReportFoundIssues(report)) {
        // Resetting here handles a situation where issues are found, then a second run finds no
        // issues and then another run finds issues again. Without resetting after the second run,
        // that last run would not send a notification
        resetHasActedOnPreviousNotification();
        return;
      }

      if (isConnectionsPanelOpenRef.current) {
        // If the connection panel is open and the report has found some issues, the user should be
        // able to see the warning in the panel. If they're on the VNet panel, we don't want to show
        // the notification _and_ the alert in the panel.
        //
        // diagNotificationIdRef needs to be made dirty so that on the next run the notification is
        // considered to be acted upon.
        diagNotificationIdRef.current = 'bogus-id';
        return;
      }

      diagNotificationIdRef.current = notificationsService.notifyWarning({
        isAutoRemovable: false,
        title: 'Other software on your device might interfere with VNet.',
        action: {
          content: 'Open Diag Report',
          onClick: () => {
            openReport(report);
            // NOTE: Do not reset diagNotificationIdRef here. Opening a notification must result in
            // hasActedOnPreviousNotification to be equal to true on the next interval run.
            notificationsService.removeNotification(
              diagNotificationIdRef.current
            );
          },
        },
      });
    },
    [
      runDiagnostics,
      notificationsService,
      openReport,
      hasDismissedDiagnosticsAlertRef,
      resetHasActedOnPreviousNotification,
      isConnectionsPanelOpenRef,
    ]
  );

  useEffect(
    function periodicallyRunDiagnostics() {
      if (!configService.get('unstable.vnetDiag').value) {
        return;
      }

      if (status.value !== 'running') {
        return;
      }

      let abortController = new AbortController();

      runDiagnosticsAndShowNotification(abortController.signal);
      const intervalId = setInterval(() => {
        abortController.abort();
        abortController = new AbortController();

        runDiagnosticsAndShowNotification(abortController.signal);
      }, diagnosticsIntervalMs);

      return () => {
        abortController.abort();
        clearInterval(intervalId);
        resetHasActedOnPreviousNotification();
      };
    },
    [
      configService,
      diagnosticsIntervalMs,
      runDiagnosticsAndShowNotification,
      status.value,
      resetHasActedOnPreviousNotification,
    ]
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
        getDisabledDiagnosticsReason,
        dismissDiagnosticsAlert,
        hasDismissedDiagnosticsAlert,
        reinstateDiagnosticsAlert,
        openReport: rootClusterUri ? openReport : undefined,
      }}
    >
      {children}
    </VnetContext.Provider>
  );
};

const defaultDiagnosticsIntervalMs = 30 * 1000; // 30s

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
