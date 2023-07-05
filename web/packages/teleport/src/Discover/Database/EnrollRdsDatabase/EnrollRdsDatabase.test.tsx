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
import { render, screen, fireEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import {
  AwsRdsDatabase,
  integrationService,
} from 'teleport/services/integrations';
import { createTeleportContext } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';
import TeleportContext from 'teleport/teleportContext';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';

import { EnrollRdsDatabase } from './EnrollRdsDatabase';

describe('test EnrollRdsDatabase.tsx', () => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {} as any,
    currentStep: 0,
    nextStep: jest.fn(x => x),
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      dbMeta: {
        location: DatabaseLocation.Aws,
        engine: DatabaseEngine.AuroraMysql,
      },
    } as any,
    viewConfig: null,
    exitFlow: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: jest.fn(x => x),
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  beforeEach(() => {
    jest
      .spyOn(ctx.databaseService, 'fetchDatabases')
      .mockResolvedValue({ agents: [] });
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('without rds database result, does not attempt to fetch db servers', async () => {
    renderRdsDatabase(ctx, discoverCtx);
    jest
      .spyOn(integrationService, 'fetchAwsRdsDatabases')
      .mockResolvedValue({ databases: [] });

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    // No results are rendered.
    await screen.findByText(/no result/i);

    expect(integrationService.fetchAwsRdsDatabases).toHaveBeenCalledTimes(1);
    expect(ctx.databaseService.fetchDatabases).not.toHaveBeenCalled();
  });

  test('with rds database result, makes a fetch request for db servers', async () => {
    renderRdsDatabase(ctx, discoverCtx);
    jest.spyOn(integrationService, 'fetchAwsRdsDatabases').mockResolvedValue({
      databases: mockAwsDbs,
    });

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    // Rds results renders result.
    await screen.findByText(/rds-1/i);

    expect(integrationService.fetchAwsRdsDatabases).toHaveBeenCalledTimes(1);
    expect(ctx.databaseService.fetchDatabases).toHaveBeenCalledTimes(1);
  });
});

function renderRdsDatabase(
  ctx: TeleportContext,
  discoverCtx: DiscoverContextState
) {
  return render(
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'database' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <FeaturesContextProvider value={[]}>
          <DiscoverProvider mockCtx={discoverCtx}>
            <EnrollRdsDatabase />
          </DiscoverProvider>
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

const mockAwsDbs: AwsRdsDatabase[] = [
  {
    engine: 'postgres',
    name: 'rds-1',
    uri: 'endpoint-1',
    status: 'available',
    labels: [{ name: 'env', value: 'prod' }],
    accountId: 'account-id-1',
    resourceId: 'resource-id-1',
  },
];
