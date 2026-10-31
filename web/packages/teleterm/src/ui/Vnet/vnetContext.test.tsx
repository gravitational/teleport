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

import { act, renderHook, waitFor } from '@testing-library/react';
import {
  ComponentType,
  createRef,
  PropsWithChildren,
  RefObject,
  useEffect,
  useImperativeHandle,
} from 'react';

import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { WindowsServiceStatus } from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';
import { ToastNotificationItemObjectContent } from 'shared/components/ToastNotification/types';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import {
  makeReport,
  makeReportWithIssuesFound,
} from 'teleterm/services/vnet/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeDocumentConnectMyComputer } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import {
  ConnectionsContextProvider,
  useConnectionsContext,
} from 'teleterm/ui/TopBar/Connections/connectionsContext';
import { IAppContext } from 'teleterm/ui/types';
import { routing } from 'teleterm/ui/uri';

import {
  useVnetContext,
  VnetContext,
  VnetContextProvider,
  VnetStatus,
  VnetStoppedReason,
} from './vnetContext';

describe('autostart', () => {
  it('starts VNet if turned on', async () => {
    const appContext = new MockAppContext();
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: true, hasEverStarted: true },
    });

    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext }),
    });

    await waitFor(
      () => expect(result.current.startAttempt.status).toEqual('success'),
      { interval: 5 }
    );
  });

  it('waits for workspace state to be initialized', async () => {
    const appContext = new MockAppContext();
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: true, hasEverStarted: true },
    });

    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext }),
    });

    expect(result.current.startAttempt.status).toEqual('');

    act(() => {
      appContext.workspacesService.setState(draft => {
        draft.isInitialized = true;
      });
    });

    await waitFor(
      () => expect(result.current.startAttempt.status).toEqual('success'),
      { interval: 5 }
    );
  });

  it('does not start VNet if turned off', async () => {
    const appContext = new MockAppContext();
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: false, hasEverStarted: false },
    });
    const { promise, resolve } = Promise.withResolvers();
    appContext.vnet.checkInstallTimeRequirements = async () => {
      const response = new MockedUnaryCall({
        status: {
          oneofKind: undefined,
        },
      });
      resolve(response);
      return response;
    };
    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext }),
    });
    await act(() => promise);

    expect(result.current.startAttempt.status).toEqual('');
  });

  it('does not start VNet if Windows system service does not exist', async () => {
    const appContext = new MockAppContext();
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: true, hasEverStarted: true },
    });
    const { promise, resolve } = Promise.withResolvers();
    appContext.vnet.checkInstallTimeRequirements = async () => {
      const response = new MockedUnaryCall({
        status: {
          oneofKind: 'windowsServiceStatus' as const,
          windowsServiceStatus: WindowsServiceStatus.DOES_NOT_EXIST,
        },
      });
      resolve(response);
      return response;
    };
    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext }),
    });
    await act(() => promise);

    expect(result.current.startAttempt.status).toEqual('');
  });

  it('does not start VNet if Windows system service version does not match client version', async () => {
    const appContext = new MockAppContext();
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: true, hasEverStarted: true },
    });
    const { promise, resolve } = Promise.withResolvers();
    appContext.vnet.checkInstallTimeRequirements = async () => {
      const response = new MockedUnaryCall({
        status: {
          oneofKind: 'windowsServiceStatus' as const,
          windowsServiceStatus: WindowsServiceStatus.VERSION_MISMATCH,
        },
      });
      resolve(response);
      return response;
    };
    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext }),
    });
    await act(() => promise);

    expect(result.current.startAttempt.status).toEqual('');
    expect(result.current.installTimeRequirementsCheck).toEqual({
      status: 'failed',
      reason: {
        kind: 'windows-service-version-mismatch',
      },
    });
  });

  it('switches off if start fails', async () => {
    const appContext = new MockAppContext();
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    const { statePersistenceService } = appContext;
    statePersistenceService.putState({
      ...statePersistenceService.getState(),
      vnet: { autoStart: true, hasEverStarted: true },
    });
    jest
      .spyOn(appContext.vnet, 'start')
      .mockRejectedValue(new MockedUnaryCall({}));

    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext }),
    });

    await waitFor(
      () => expect(result.current.startAttempt.status).toEqual('error'),
      { interval: 5 }
    );

    expect(statePersistenceService.getState().vnet.autoStart).toEqual(false);
  });

  test('starting and stopping VNet toggles autostart', async () => {
    const appContext = new MockAppContext();
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    const { statePersistenceService } = appContext;
    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext }),
    });

    expect(statePersistenceService.getState()?.vnet?.autoStart).not.toBe(true);

    let err: Error;

    await act(async () => {
      [, err] = await result.current.start();
    });
    expect(err).toBeFalsy();
    expect(statePersistenceService.getState().vnet.autoStart).toEqual(true);

    await act(async () => {
      [, err] = await result.current.stop();
    });
    expect(err).toBeFalsy();
    expect(statePersistenceService.getState().vnet.autoStart).toEqual(false);
  });
});

it('registers a callback for unexpected shutdown', async () => {
  const appContext = new MockAppContext();
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true, hasEverStarted: true },
  });

  const { result } = renderHook(() => useVnetContext(), {
    wrapper: createWrapper(Wrapper, { appContext }),
  });

  await waitFor(
    () => expect(result.current.startAttempt.status).toEqual('success'),
    { interval: 5 }
  );

  // Trigger unexpected shutdown.
  act(() => {
    appContext.unexpectedVnetShutdownListener({
      error: 'lorem ipsum dolor sit amet',
    });
  });

  await waitFor(
    () => {
      expect(result.current.status.value).toEqual('stopped');
    },
    { interval: 5 }
  );

  const status = result.current.status as Extract<
    VnetStatus,
    { value: 'stopped' }
  >;
  expect(status.reason.value).toEqual('unexpected-shutdown');
  const reason = status.reason as Extract<
    VnetStoppedReason,
    { value: 'unexpected-shutdown' }
  >;
  expect(reason.errorMessage).toEqual('lorem ipsum dolor sit amet');
});

describe('session expired', () => {
  const setup = async (
    controlConnectionsRef?: RefObject<ControlConnections>
  ) => {
    const appContext = new MockAppContext();
    const cluster = makeRootCluster({
      loggedInUser: makeLoggedInUser({
        validUntil: Timestamp.fromDate(new Date(Date.now() - 60_000)),
      }),
    });
    appContext.addRootCluster(cluster);
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: true, hasEverStarted: true },
    });

    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext, controlConnectionsRef }),
    });
    await waitFor(
      () => expect(result.current.startAttempt.status).toEqual('success'),
      { interval: 5 }
    );

    return { appContext, cluster, result };
  };

  const getSessionExpiredNotifications = (appContext: MockAppContext) =>
    appContext.notificationsService.getNotifications();

  const renewCert = (appContext: MockAppContext, cluster: Cluster) =>
    appContext.clustersService.setState(draft => {
      draft.clusters.get(cluster.uri).loggedInUser.validUntil =
        Timestamp.fromDate(new Date(Date.now() + 60 * 60_000));
    });

  it('notifies once per expiry', async () => {
    const { appContext, cluster } = await setup();

    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'foo.example.com',
    });
    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'bar.example.com',
    });

    expect(getSessionExpiredNotifications(appContext)).toHaveLength(1);
    expect(getSessionExpiredNotifications(appContext)[0].content).toEqual(
      expect.objectContaining({
        title: `Your session for ${routing.parseClusterName(cluster.uri)} has expired`,
      })
    );

    // Signing in removes the notification.
    renewCert(appContext, cluster);
    await waitFor(() =>
      expect(getSessionExpiredNotifications(appContext)).toHaveLength(0)
    );

    // Expiring again after signing in notifies again.
    appContext.clustersService.setState(draft => {
      draft.clusters.get(cluster.uri).loggedInUser.validUntil =
        Timestamp.fromDate(new Date(Date.now() - 1));
    });
    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'foo.example.com',
    });
    expect(getSessionExpiredNotifications(appContext)).toHaveLength(1);
  });

  it('removes the notification after ten minutes', async () => {
    const { appContext, cluster } = await setup();
    jest.useFakeTimers();

    try {
      appContext.vnetSessionExpiredListener({
        rootClusterUris: [cluster.uri],
        hostname: 'foo.example.com',
      });
      expect(getSessionExpiredNotifications(appContext)).toHaveLength(1);

      jest.advanceTimersByTime(10 * 60 * 1000);

      expect(getSessionExpiredNotifications(appContext)).toHaveLength(0);
    } finally {
      jest.useRealTimers();
    }
  });

  it('notifies again after the previous notification auto-dismissed', async () => {
    const { appContext, cluster } = await setup();
    jest.useFakeTimers();

    try {
      appContext.vnetSessionExpiredListener({
        rootClusterUris: [cluster.uri],
        hostname: 'foo.example.com',
      });
      expect(getSessionExpiredNotifications(appContext)).toHaveLength(1);

      // The notification auto-dismisses after ten minutes without the user acting on it.
      jest.advanceTimersByTime(10 * 60 * 1000);
      expect(getSessionExpiredNotifications(appContext)).toHaveLength(0);

      // A later connection attempt during the same expiry episode (cert still not renewed)
      // should notify again instead of staying silent forever.
      appContext.vnetSessionExpiredListener({
        rootClusterUris: [cluster.uri],
        hostname: 'bar.example.com',
      });
      expect(getSessionExpiredNotifications(appContext)).toHaveLength(1);
    } finally {
      jest.useRealTimers();
    }
  });

  it('refreshes the hostname in the description while the notification is still visible', async () => {
    const { appContext, cluster } = await setup();

    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'foo.example.com',
    });
    expect(getSessionExpiredNotifications(appContext)).toHaveLength(1);
    expect(getSessionExpiredNotifications(appContext)[0].content).toEqual(
      expect.objectContaining({
        description: expect.stringContaining('foo.example.com'),
      })
    );

    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'bar.example.com',
    });

    // Still a single notification, but its description now reflects the latest attempt.
    expect(getSessionExpiredNotifications(appContext)).toHaveLength(1);
    expect(getSessionExpiredNotifications(appContext)[0].content).toEqual(
      expect.objectContaining({
        description: expect.stringContaining('bar.example.com'),
      })
    );
  });

  it('does not resurface a notification that the user dismissed via the close button', async () => {
    const { appContext, cluster } = await setup();

    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'foo.example.com',
    });
    const [notification] = getSessionExpiredNotifications(appContext);

    // Simulate the toast calling its onRemove callback, as it does when the user clicks the close
    // button.
    (notification.content as ToastNotificationItemObjectContent).onRemove();
    expect(getSessionExpiredNotifications(appContext)).toHaveLength(0);

    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'bar.example.com',
    });
    expect(getSessionExpiredNotifications(appContext)).toHaveLength(0);
  });

  it('adds a notification for each cluster in the event', async () => {
    const { appContext, cluster } = await setup();
    const otherCluster = makeRootCluster({
      uri: '/clusters/other.example.com',
      name: 'other.example.com',
      proxyHost: 'other.example.com:3080',
      loggedInUser: makeLoggedInUser({
        validUntil: Timestamp.fromDate(new Date(Date.now() - 60_000)),
      }),
    });
    appContext.addRootCluster(otherCluster);

    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri, otherCluster.uri],
      hostname: 'foo.example.com',
    });

    expect(getSessionExpiredNotifications(appContext)).toHaveLength(2);
  });

  it('removes the notification when VNet stops', async () => {
    const { appContext, cluster, result } = await setup();

    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'foo.example.com',
    });
    expect(getSessionExpiredNotifications(appContext)).toHaveLength(1);

    await act(() => result.current.stop());

    expect(getSessionExpiredNotifications(appContext)).toHaveLength(0);
  });

  it('does not notify if the cert is already valid by the time the report arrives', async () => {
    const { appContext, cluster } = await setup();

    // The cert gets renewed (e.g. through another sign-in path) before this report, which was
    // generated while the session was still expired, is actually delivered and processed.
    renewCert(appContext, cluster);

    appContext.vnetSessionExpiredListener({
      rootClusterUris: [cluster.uri],
      hostname: 'foo.example.com',
    });

    expect(getSessionExpiredNotifications(appContext)).toHaveLength(0);
  });
});

describe('diag notification', () => {
  const noIssuesFoundReport = makeReport();
  const issuesFoundReport = makeReportWithIssuesFound();
  const tests: Array<{
    it: string;
    /** Ref for opening/closing the connections panel. If provided, the panel will be open by default. */
    controlConnectionsRef?: RefObject<ControlConnections>;
    mockAppContext: (appContext: MockAppContext) => void;
    verify: (
      appContext: MockAppContext,
      result: { current: VnetContext },
      controlConnectionsRef?: RefObject<ControlConnections>
    ) => Promise<void>;
  }> = [
    {
      it: 'is shown, closed, then shown again when the report cycles from issues found to no issues and back to issues found',
      mockAppContext: appContext => {
        jest
          .spyOn(appContext.vnet, 'runDiagnostics')
          .mockResolvedValueOnce(
            new MockedUnaryCall({ report: issuesFoundReport })
          )
          .mockResolvedValueOnce(
            new MockedUnaryCall({ report: noIssuesFoundReport })
          )
          .mockResolvedValueOnce(
            new MockedUnaryCall({ report: issuesFoundReport })
          )
          .mockResolvedValue(
            new MockedUnaryCall({}, new Error('something went wrong'))
          );
      },
      verify: async ({ notificationsService, vnet }) => {
        // Verify that after the diagnostics are run three times, only two notifications have been
        // created.
        await waitFor(
          () => expect(vnet.runDiagnostics).toHaveBeenCalledTimes(3),
          { interval }
        );

        expect(notificationsService.notifyWarning).toHaveBeenCalledTimes(2);
        const notifications = notificationsService.getNotifications();
        expect(notifications).toHaveLength(1);
        expect(notifications[0].content).toMatchObject({
          description: undefined,
          action: expect.objectContaining({ content: 'Open Diag Report' }),
        });
      },
    },
    {
      it: 'is not shown after opening a report, receiving another report with no issues and then issues reoccuring',
      mockAppContext: appContext => {
        jest
          .spyOn(appContext.vnet, 'runDiagnostics')
          .mockReturnValue(new MockedUnaryCall({ report: issuesFoundReport }));
      },
      verify: async ({ notificationsService, vnet }, result) => {
        // Wait for the first run to create a notification.
        await waitFor(
          () => expect(notificationsService.getNotifications()).toHaveLength(1),
          { interval }
        );

        // Open the diag report and verify that it removes the notification.
        await act(async () => {
          result.current.openReport(result.current.diagnosticsAttempt.data);
        });
        expect(notificationsService.getNotifications()).toHaveLength(0);

        jest.clearAllMocks();

        // Wait for at least one no-issues run…
        jest
          .spyOn(vnet, 'runDiagnostics')
          .mockReturnValue(
            new MockedUnaryCall({ report: noIssuesFoundReport })
          );
        await waitFor(
          () => expect(vnet.runDiagnostics).toHaveBeenCalledTimes(1),
          { interval }
        );

        // …then flip back to returning issues and wait for one more run. Verify that the cycle
        // did not create another notification.
        jest
          .spyOn(vnet, 'runDiagnostics')
          .mockReturnValue(new MockedUnaryCall({ report: issuesFoundReport }));
        await waitFor(
          () => expect(vnet.runDiagnostics).toHaveBeenCalledTimes(2),
          { interval }
        );
        expect(notificationsService.notifyWarning).toHaveBeenCalledTimes(0);
        expect(notificationsService.getNotifications()).toHaveLength(0);
      },
    },
    {
      it: "is not shown when the VNet panel is opened and doesn't appear after the panel is closed",
      controlConnectionsRef: createRef(),
      mockAppContext: appContext => {
        jest
          .spyOn(appContext.vnet, 'runDiagnostics')
          .mockResolvedValue(
            new MockedUnaryCall({ report: issuesFoundReport })
          );
      },
      verify: async (
        { vnet, notificationsService },
        result,
        controlConnectionsRef
      ) => {
        await waitFor(
          () =>
            expect(result.current.diagnosticsAttempt.status).toEqual('success'),
          { interval }
        );
        expect(notificationsService.notifyWarning).not.toHaveBeenCalled();

        // Close the panel and wait for the next run, verify that the notification wasn't sent.
        await act(async () => controlConnectionsRef.current.close());
        await waitFor(
          () => expect(vnet.runDiagnostics).toHaveBeenCalledTimes(2),
          { interval }
        );
        expect(notificationsService.notifyWarning).not.toHaveBeenCalled();
      },
    },
    {
      it: 'is not shown when the VNet panel is opened and no issues are found, but appears after the panel is closed and issues are found',
      controlConnectionsRef: createRef(),
      mockAppContext: appContext => {
        jest
          .spyOn(appContext.vnet, 'runDiagnostics')
          .mockReturnValue(
            new MockedUnaryCall({ report: noIssuesFoundReport })
          );
      },
      verify: async (
        { vnet, notificationsService },
        result,
        controlConnectionsRef
      ) => {
        await waitFor(
          () =>
            expect(result.current.diagnosticsAttempt.status).toEqual('success'),
          { interval }
        );
        expect(notificationsService.getNotifications()).toHaveLength(0);

        // Close the panel and start returning report with issues, then wait for the next run to
        // produce a notification.
        await act(async () => controlConnectionsRef.current.close());
        jest
          .spyOn(vnet, 'runDiagnostics')
          .mockReturnValue(new MockedUnaryCall({ report: issuesFoundReport }));
        await waitFor(
          () => expect(notificationsService.getNotifications()).toHaveLength(1),
          { interval }
        );
      },
    },
    {
      it: 'is not shown after dismissing a dialog and reappears after manually running diagnostics',
      mockAppContext: appContext => {
        jest
          .spyOn(appContext.vnet, 'runDiagnostics')
          .mockResolvedValue(
            new MockedUnaryCall({ report: issuesFoundReport })
          );
      },
      verify: async ({ notificationsService, vnet }, result) => {
        // Verify that the first run creates a notification.
        await waitFor(
          () =>
            expect(result.current.diagnosticsAttempt.status).toEqual('success'),
          { interval }
        );
        expect(notificationsService.getNotifications()).toHaveLength(1);

        // Verify that dismissing the alert removes the notification.
        await act(async () => result.current.dismissDiagnosticsAlert());
        expect(notificationsService.getNotifications()).toHaveLength(0);

        // Verify that the next auto diagnostics run does not create a notification.
        jest.clearAllMocks();
        await waitFor(
          () => expect(vnet.runDiagnostics).toHaveBeenCalledTimes(1),
          { interval }
        );
        expect(notificationsService.getNotifications()).toHaveLength(0);

        // Manually run diagnostics, wait for another auto call and confirm that it creates a
        // notification.
        await act(() =>
          result.current
            .runDiagnostics()
            .finally(() => result.current.reinstateDiagnosticsAlert())
        );
        jest.clearAllMocks();
        await waitFor(
          () => expect(vnet.runDiagnostics).toHaveBeenCalledTimes(1),
          { interval }
        );
        expect(notificationsService.getNotifications()).toHaveLength(1);
      },
    },
    {
      it: 'does not show a button to open the diag report if there is no workspace',
      mockAppContext: appContext => {
        jest
          .spyOn(appContext.vnet, 'runDiagnostics')
          .mockResolvedValue(
            new MockedUnaryCall({ report: issuesFoundReport })
          );
        appContext.workspacesService.setState(draft => {
          draft.rootClusterUri = undefined;
        });
      },
      verify: async ({ notificationsService }, result) => {
        await waitFor(
          () =>
            expect(result.current.diagnosticsAttempt.status).toEqual('success'),
          { interval }
        );
        const notifications = notificationsService.getNotifications();
        expect(notifications).toHaveLength(1);
        expect(notifications[0].content).toMatchObject({
          description: expect.stringContaining('Log in to a cluster'),
          action: undefined,
        });
      },
    },
  ];

  test.each(tests)('$it', async test => {
    const appContext = new MockAppContext();
    // Set up a proper workspace so that the diag report can be opened.
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    appContext.addRootClusterWithDoc(
      makeRootCluster(),
      makeDocumentConnectMyComputer()
    );
    // Automatically start VNet.
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: true, hasEverStarted: true },
    });

    jest.spyOn(appContext.notificationsService, 'notifyWarning');

    test.mockAppContext(appContext);

    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, {
        appContext,
        controlConnectionsRef: test.controlConnectionsRef,
      }),
    });

    await test.verify(appContext, result, test.controlConnectionsRef);
  });
});

const Wrapper = (
  props: PropsWithChildren<{
    appContext: IAppContext;
    controlConnectionsRef?: RefObject<ControlConnections>;
  }>
) => {
  return (
    <MockAppContextProvider appContext={props.appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider diagnosticsIntervalMs={diagnosticsIntervalMs}>
          {props.controlConnectionsRef && (
            <OpenConnections
              controlConnectionsRef={props.controlConnectionsRef}
            />
          )}
          {props.children}
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );
};

const diagnosticsIntervalMs = 50;
/** Interval for waitFor. Needs to be lower than diagnosticsIntervalMs. */
const interval = 10;

// Make Wrapper receive initialProps.
// https://testing-library.com/docs/react-testing-library/api/#renderhook-options-initialprops
function createWrapper<Props>(
  Wrapper: ComponentType<PropsWithChildren<Props>>,
  props: PropsWithChildren<Props>
) {
  return function CreatedWrapper({ children }) {
    return <Wrapper {...props}>{children}</Wrapper>;
  };
}

const OpenConnections = (props: {
  controlConnectionsRef: RefObject<ControlConnections>;
}) => {
  const { open, close } = useConnectionsContext();
  useImperativeHandle(props.controlConnectionsRef, () => ({ open, close }));

  useEffect(() => {
    open();
  }, [open]);

  return null;
};

type ControlConnections = {
  open: () => void;
  close: () => void;
};
