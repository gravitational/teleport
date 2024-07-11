import React from 'react';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';

import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';

import cfg from 'teleport/config';

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import {
  ConfigurePool,
  isValidGCPResourceName,
  isValidGcpOrgID,
} from './ConfigureWorkforcePool';

import type { SAMLIdPMetadataResponse } from 'e-teleport/services/idp/types';
import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';

describe('Configure GCP workforce pool', () => {
  const Provider = props => {
    const ctx = createTeleportContextE({ customAcl: props.customAcl });
    const discoverCtx: DiscoverContextState = {
      nextStep: () => null,
      prevStep: () => null,
      agentMeta: {
        samlGcpWorkforce: {
          isAutoConfig: true,
          orgId: '',
          poolName: '',
          poolProviderName: '',
        },
      },
      updateAgentMeta: AgentMeta => AgentMeta,
      resourceSpec: {
        samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
      } as ResourceSpec,
      currentStep: 0,
      onSelectResource: () => null,
      exitFlow: () => null,
      viewConfig: null,
      indexedViews: [],
      setResourceSpec: () => null,
      emitErrorEvent: () => null,
      emitEvent: () => null,
      eventState: null,
    };

    return (
      <MemoryRouter
        initialEntries={[
          { pathname: cfg.routes.discover, state: { entity: 'app' } },
        ]}
      >
        <ContextProvider ctx={ctx}>
          <DiscoverProvider mockCtx={discoverCtx}>
            {props.children}
          </DiscoverProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  test('Toggle on and off enables and disales auto config flow', async () => {
    const mockFetchMetadata = jest.fn().mockResolvedValue({
      entityID: '',
      ssoURL: '',
      x509PEM: '',
    } as SAMLIdPMetadataResponse);

    render(
      <Provider>
        <ConfigurePool
          agentMeta={{}}
          updateAgentMeta={jest.fn()}
          resourceSpec={
            {
              samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
            } as ResourceSpec
          }
          prevStep={() => null}
          fetchMetadata={mockFetchMetadata}
          nextStep={() => null}
        />
      </Provider>
    );

    const guidedFlowEl = screen.getByText(
      'Guided configuration flow is enabled.'
    );
    expect(guidedFlowEl).toBeInTheDocument();

    expect(
      screen.getByText(
        'Generate an installation command to configure Workforce Identity Federation pool and pool provider'
      )
    ).toBeInTheDocument();

    fireEvent.click(guidedFlowEl);

    await waitFor(() => {
      expect(
        screen.getByText('Guided configuration flow is disabled.')
      ).toBeInTheDocument();
    });

    expect(
      screen.getByText('Guided configuration flow is disabled.')
    ).toBeInTheDocument();

    expect(screen.getByText('Teleport IdP Metadata')).toBeInTheDocument();
  });
});

describe('isValidGcpOrgID', () => {
  test.each`
    orgId                  | valid
    ${'1234567890'}        | ${true}
    ${'123asbdac908'}      | ${false}
    ${'abc-123-qwe99'}     | ${false}
    ${'123458132-1234123'} | ${false}
    ${'1abc-123-qwe'}      | ${false}
  `('organization Id: $orgId', ({ orgId, valid }) => {
    const result = isValidGcpOrgID(orgId)();

    expect(result.valid).toEqual(valid);
  });
});

describe('isValidGCPResourceName', () => {
  test.each`
    names                                                                                 | valid
    ${'abc-asd-qwe'}                                                                      | ${true}
    ${'abc-123-qwe'}                                                                      | ${true}
    ${'abc-123-qwe99'}                                                                    | ${true}
    ${'abc-123-qweabc-123-qabc-123-qweabc-123-qabc-123-qweabc-123-qabc-123-qweabc-123-q'} | ${false}
    ${'abc-ABC-123-qwe'}                                                                  | ${false}
    ${'Aabc-123-qwe'}                                                                     | ${false}
    ${'abc-1_23-qwe'}                                                                     | ${false}
    ${'abc-123-qwe-'}                                                                     | ${false}
    ${'1abc-123-qwe'}                                                                     | ${false}
  `('names: $names', ({ names, valid }) => {
    const result = isValidGCPResourceName(names)();

    expect(result.valid).toEqual(valid);
  });
});
