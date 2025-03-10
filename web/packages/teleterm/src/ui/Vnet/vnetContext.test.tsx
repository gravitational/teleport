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
  MutableRefObject,
  PropsWithChildren,
  useEffect,
  useImperativeHandle,
} from 'react';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
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
      vnet: { autoStart: true },
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
      vnet: { autoStart: true },
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
      vnet: { autoStart: false },
    });

    const { result } = renderHook(() => useVnetContext(), {
      wrapper: createWrapper(Wrapper, { appContext }),
    });

    expect(result.current.startAttempt.status).toEqual('');
  });

  it('switches off if start fails', async () => {
    const appContext = new MockAppContext();
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
    const { statePersistenceService } = appContext;
    statePersistenceService.putState({
      ...statePersistenceService.getState(),
      vnet: { autoStart: true },
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
    vnet: { autoStart: true },
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

/* eslint-disable jest/no-standalone-expect */
describe('diag notification', () => {
  const noIssuesFoundReport = makeReport();
  const issuesFoundReport = makeReportWithIssuesFound();
  const tests: Array<{
    it: string;
    /** Ref for opening/closing the connections panel. If provided, the panel will be open by default. */
    controlConnectionsRef?: MutableRefObject<ControlConnections>;
    mockAppContext: (appContext: MockAppContext) => void;
    verify: (
      appContext: MockAppContext,
      result: { current: VnetContext },
      controlConnectionsRef?: MutableRefObject<ControlConnections>
    ) => Promise<void>;
  }> = [
    {
      it: 'is shown when the report cycles from issues found to no issues and back to issues found',
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
        expect(notificationsService.getNotifications()).toHaveLength(1);
      },
    },
    {
      it: 'is not shown after opening a report, receiving another report with no issues and then issues reoccuring',
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
      verify: async ({ notificationsService, vnet }, result) => {
        // Open the diag report and verify that it removes the notification.
        await waitFor(
          () =>
            expect(result.current.diagnosticsAttempt.status).toEqual('success'),
          { interval }
        );
        await act(async () => {
          result.current.openReport(result.current.diagnosticsAttempt.data);
        });
        expect(notificationsService.notifyWarning).toHaveBeenCalledTimes(1);
        expect(notificationsService.getNotifications()).toHaveLength(0);

        jest.clearAllMocks();

        // Wait for the third report to be processed and verify that it does not result in another
        // notification being created.
        await waitFor(
          () => expect(vnet.runDiagnostics).toHaveBeenCalledTimes(3),
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
          .mockResolvedValueOnce(
            new MockedUnaryCall({ report: noIssuesFoundReport })
          )
          .mockReturnValue(new MockedUnaryCall({ report: issuesFoundReport }));
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

        // Close the panel and wait for the next run, verify that the notification was sent.
        await act(async () => controlConnectionsRef.current.close());
        await waitFor(
          () => expect(vnet.runDiagnostics).toHaveBeenCalledTimes(2),
          { interval }
        );
        expect(notificationsService.getNotifications().length).toEqual(1);
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
  ];

  // eslint-disable-next-line jest/expect-expect
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
      vnet: { autoStart: true },
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
/* eslint-enable jest/no-standalone-expect */

const Wrapper = (
  props: PropsWithChildren<{
    appContext: IAppContext;
    controlConnectionsRef?: MutableRefObject<ControlConnections>;
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
  controlConnectionsRef: MutableRefObject<ControlConnections>;
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
