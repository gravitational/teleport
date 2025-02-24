/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { act, renderHook } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import api from 'teleport/services/api';
import {
  DiscoverEvent,
  DiscoverEventResource,
  DiscoverEventStatus,
  DiscoverServiceDeployMethod,
  DiscoverServiceDeployType,
  userEventService,
} from 'teleport/services/userEvent';

import { SERVERS } from './SelectResource/resources';
import { DiscoverProvider, useDiscover } from './useDiscover';

describe('emitting events', () => {
  const ctx = createTeleportContext();
  let wrapper;

  beforeEach(() => {
    jest.spyOn(api, 'get').mockResolvedValue([]); // required for fetchClusterAlerts
    jest
      .spyOn(userEventService, 'captureDiscoverEvent')
      .mockResolvedValue(null as never); // return value does not matter but required by ts

    wrapper = ({ children }) => (
      <MemoryRouter initialEntries={[{ pathname: cfg.routes.discover }]}>
        <ContextProvider ctx={ctx}>
          <FeaturesContextProvider value={[]}>
            <DiscoverProvider>{children}</DiscoverProvider>
          </FeaturesContextProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  test('first render, init event state and emits started event', async () => {
    const { result } = renderHook(() => useDiscover(), {
      wrapper,
    });

    // Test init hook.
    expect(result.current.currentStep).toBe(0);
    expect(result.current.eventState).toEqual(
      expect.objectContaining({
        id: expect.any(String),
        currEventName: DiscoverEvent.Started,
      })
    );

    const eventId = result.current.eventState.id;
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledTimes(1);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: DiscoverEvent.Started,
        eventData: {
          id: eventId,
          resource: '',
          stepStatus: DiscoverEventStatus.Success,
        },
      })
    );
    jest.resetAllMocks();
  });

  test('onSelectResource emits resource selected event', async () => {
    const { result } = renderHook(() => useDiscover(), {
      wrapper,
    });
    jest.resetAllMocks(); // discount the init event

    await act(async () => {
      result.current.onSelectResource(SERVERS[0]);
    });

    // Event state is set to the next step.
    expect(result.current.eventState).toEqual(
      expect.objectContaining({
        id: expect.any(String),
        currEventName: DiscoverEvent.DeployService,
      })
    );

    const eventId = result.current.eventState.id;
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledTimes(1);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: DiscoverEvent.ResourceSelection,
        eventData: {
          id: eventId,
          resource: DiscoverEventResource.Server,
          stepStatus: DiscoverEventStatus.Success,
        },
      })
    );
    jest.resetAllMocks();
  });

  test('incrementing view by one, emits success events', async () => {
    const { result } = renderHook(() => useDiscover(), {
      wrapper,
    });

    // Set the resources.
    await act(async () => {
      result.current.onSelectResource(SERVERS[0]);
    });
    jest.resetAllMocks(); // discount the events from init and select resource
    const id = result.current.eventState.id;

    // Test next step gets incremented by 1, passing in a non-number.
    await act(async () => {
      result.current.nextStep('non-number' as any);
    });
    expect(result.current.currentStep).toBe(1);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledTimes(1);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: DiscoverEvent.DeployService,
        eventData: {
          id,
          resource: DiscoverEventResource.Server,
          stepStatus: DiscoverEventStatus.Success,
          serviceDeploy: {
            method: DiscoverServiceDeployMethod.Unspecified,
            type: DiscoverServiceDeployType.Unspecified,
          },
        },
      })
    );
    // Test the `eventState` got updated to the next view.
    expect(result.current.eventState).toEqual({
      id,
      currEventName: DiscoverEvent.PrincipalsConfigure,
    });
    jest.resetAllMocks();

    // Test passing in nothing, increments by 1.
    await act(async () => {
      result.current.nextStep();
    });
    expect(result.current.currentStep).toBe(2);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledTimes(1);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: DiscoverEvent.PrincipalsConfigure,
        eventData: {
          id,
          resource: DiscoverEventResource.Server,
          stepStatus: DiscoverEventStatus.Success,
        },
      })
    );
  });

  test('programatically skipping, emits skipped and one success event', async () => {
    const { result } = renderHook(() => useDiscover(), {
      wrapper,
    });

    // Set the resources.
    await act(async () => {
      result.current.onSelectResource(SERVERS[0]);
    });
    jest.resetAllMocks(); // discount the events from init and select resource

    const id = result.current.eventState.id;

    // Test all skipped steps have been emitted.
    await act(async () => {
      result.current.nextStep(3);
    });
    expect(result.current.currentStep).toBe(3);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledTimes(3);

    // Emit the current event.
    expect(userEventService.captureDiscoverEvent).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        event: DiscoverEvent.DeployService,
        eventData: {
          id,
          resource: DiscoverEventResource.Server,
          stepStatus: DiscoverEventStatus.Success,
          serviceDeploy: {
            method: DiscoverServiceDeployMethod.Unspecified,
            type: DiscoverServiceDeployType.Unspecified,
          },
        },
      })
    );

    // Should have two skipped events.
    expect(userEventService.captureDiscoverEvent).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        event: DiscoverEvent.PrincipalsConfigure,
        eventData: {
          id,
          resource: DiscoverEventResource.Server,
          stepStatus: DiscoverEventStatus.Skipped,
        },
      })
    );
    expect(userEventService.captureDiscoverEvent).toHaveBeenNthCalledWith(
      3,
      expect.objectContaining({
        event: DiscoverEvent.TestConnection,
        eventData: {
          id,
          resource: DiscoverEventResource.Server,
          stepStatus: DiscoverEventStatus.Skipped,
        },
      })
    );
  });

  test('user intentionally skipping, emits only skipped event', async () => {
    const { result } = renderHook(() => useDiscover(), {
      wrapper,
    });
    // Set the resources.
    await act(async () => {
      result.current.onSelectResource(SERVERS[0]);
    });
    jest.resetAllMocks(); // discount the events from init and select resource

    const id = result.current.eventState.id;

    await act(async () => {
      result.current.nextStep(0);
    });
    expect(result.current.currentStep).toBe(1);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledTimes(1);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: DiscoverEvent.DeployService,
        eventData: {
          id,
          resource: DiscoverEventResource.Server,
          stepStatus: DiscoverEventStatus.Skipped,
          serviceDeploy: {
            method: DiscoverServiceDeployMethod.Unspecified,
            type: DiscoverServiceDeployType.Unspecified,
          },
        },
      })
    );
  });

  test('error event', async () => {
    const { result } = renderHook(() => useDiscover(), {
      wrapper,
    });
    // Set the resources.
    await act(async () => {
      result.current.onSelectResource(SERVERS[0]);
    });
    jest.resetAllMocks(); // discount the events from init and select resource

    await act(async () => {
      result.current.emitErrorEvent('some error message');
    });
    expect(result.current.currentStep).toBe(0);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledTimes(1);
    expect(userEventService.captureDiscoverEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: DiscoverEvent.DeployService,
        eventData: {
          id: result.current.eventState.id,
          resource: DiscoverEventResource.Server,
          stepStatus: DiscoverEventStatus.Error,
          stepStatusError: 'some error message',
          selectedResourcesCount: 0,
          autoDiscoverResourcesCount: 0,
          serviceDeploy: {
            method: DiscoverServiceDeployMethod.Unspecified,
            type: DiscoverServiceDeployType.Unspecified,
          },
        },
      })
    );
  });
});
