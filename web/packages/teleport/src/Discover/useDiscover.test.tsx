/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { MemoryRouter } from 'react-router';
import { renderHook, act } from '@testing-library/react-hooks';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import {
  DiscoverEvent,
  DiscoverEventResource,
  DiscoverEventStatus,
  DiscoverServiceDeployMethod,
  userEventService,
} from 'teleport/services/userEvent';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import api from 'teleport/services/api';
import cfg from 'teleport/config';

import { useDiscover, DiscoverProvider } from './useDiscover';
import { SERVERS } from './SelectResource/resources';

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
          serviceDeployedMethod: DiscoverServiceDeployMethod.Unspecified,
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
          serviceDeployedMethod: DiscoverServiceDeployMethod.Unspecified,
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
          serviceDeployedMethod: DiscoverServiceDeployMethod.Unspecified,
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
          serviceDeployedMethod: DiscoverServiceDeployMethod.Unspecified,
        },
      })
    );
  });
});
