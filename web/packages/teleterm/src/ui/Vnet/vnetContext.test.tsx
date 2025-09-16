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
import { PropsWithChildren } from 'react';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { IAppContext } from 'teleterm/ui/types';

import {
  useVnetContext,
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

const Wrapper = (props: PropsWithChildren<{ appContext: IAppContext }>) => (
  <MockAppContextProvider appContext={props.appContext}>
    <VnetContextProvider>{props.children}</VnetContextProvider>
  </MockAppContextProvider>
);

//testing-library.com/docs/react-testing-library/api/#renderhook-options-initialprops
const createWrapper = (Wrapper, props) => {
  return function CreatedWrapper({ children }) {
    return <Wrapper {...props}>{children}</Wrapper>;
  };
};
